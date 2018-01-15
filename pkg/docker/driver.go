package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/tonistiigi/fifo"

	protoio "github.com/gogo/protobuf/io"

	"github.com/rchicoli/docker-log-logstash/pkg/logstash"
)

const (
	name = "logstashlog"
)

type Driver struct {
	mu   sync.Mutex
	logs map[string]*container

	client *logstash.Logstash

	file File
}

type container struct {
	stream io.ReadCloser
	info   logger.Info
}

type File struct {
	fd     *os.File
	writer *bufio.Writer
	rename bool
}

type LogMessage struct {
	logdriver.LogEntry
	logger.Info
}

func (l LogMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {

			// docker/daemon/logger/Info
			Config              map[string]string `json:"config,omitempty"`
			ContainerID         string            `json:"containerID,omitempty"`
			ContainerName       string            `json:"containerName,omitempty"`
			ContainerEntrypoint string            `json:"containerEntrypoint,omitempty"`
			ContainerArgs       []string          `json:"containerArgs,omitempty"`
			ContainerImageID    string            `json:"containerImageID,omitempty"`
			ContainerImageName  string            `json:"containerImageName,omitempty"`
			ContainerCreated    *time.Time        `json:"containerCreated,omitempty"`
			ContainerEnv        []string          `json:"containerEnv,omitempty"`
			ContainerLabels     map[string]string `json:"containerLabels,omitempty"`
			LogPath             string            `json:"logPath,omitempty"`
			DaemonName          string            `json:"daemonName,omitempty"`

			//  api/types/plugin/logdriver/LogEntry
			Line     string    `json:"message"` // []byte to string
			Source   string    `json:"source"`
			TimeNano time.Time `json:"timestamp"` // int64 to Time
			Partial  bool      `json:"partial"`
		}{
			Config:              l.Config,
			ContainerID:         l.ContainerID,
			ContainerName:       l.ContainerName,
			ContainerEntrypoint: l.ContainerEntrypoint,
			ContainerArgs:       l.ContainerArgs,
			ContainerImageID:    l.ContainerImageID,
			ContainerImageName:  l.ContainerImageName,
			ContainerCreated:    &l.ContainerCreated,
			ContainerEnv:        l.ContainerEnv,
			ContainerLabels:     l.ContainerLabels,
			LogPath:             l.LogPath,
			DaemonName:          l.DaemonName,

			Line:     strings.TrimSpace(string(l.Line)),
			Source:   l.Source,
			TimeNano: time.Unix(0, l.TimeNano),
			Partial:  l.Partial,
		})

}

func NewDriver() LogDriver {

	return &Driver{
		logs: make(map[string]*container),
	}
}

func (d *Driver) StartLogging(pipe string, info logger.Info) error {

	// get config from log-opt and validate it
	cfg := defaultLogOpt()
	if err := cfg.validateLogOpt(info.Config); err != nil {
		return errors.Wrapf(err, "error: logstash-options: %q", err)
	}

	d.mu.Lock()
	if _, exists := d.logs[pipe]; exists {
		d.mu.Unlock()
		return fmt.Errorf("logger for %q already exists", pipe)
	}
	d.mu.Unlock()

	ctx := context.Background()

	// pipe file for streaming the logs
	logrus.WithField("id", info.ContainerID).WithField("pipe", pipe).WithField("logpath", info.LogPath).Debugf("Start logging")
	fifo, err := fifo.OpenFifo(ctx, pipe, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", fifo)
	}

	// cache messages to the filesystem, if the target service is not responding
	if info.LogPath == "" {
		info.LogPath = filepath.Join("/var/log/docker", info.ContainerID+".log")
	}
	if err := os.MkdirAll(filepath.Dir(info.LogPath), 0755); err != nil {
		return errors.Wrap(err, "error setting up logger dir")
	}
	f, err := d.openLogFile(info.LogPath)
	if err != nil {
		return errors.Wrapf(err, "error create cache  log file: %q", f)
	}
	d.file.fd = f

	// TODO: find a better solution
	d.mu.Lock()
	c := &container{
		stream: fifo,
		info:   info,
	}
	d.logs[pipe] = c
	d.mu.Unlock()

	d.client, err = logstash.NewClient(cfg.scheme, cfg.host, cfg.port, cfg.timeout)
	if err != nil {
		return fmt.Errorf("logstash: cannot establish a connection: %v", err)
	}

	// TODO: add context
	go d.consumeLog(c, cfg.fields)
	return nil
}

func (d *Driver) consumeLog(c *container, fields string) {

	dec := protoio.NewUint32DelimitedReader(c.stream, binary.BigEndian, 1e6)
	defer dec.Close()

	// custom log message fields
	msg := getLostashFields(fields, c.info)

	var buf logdriver.LogEntry
	for {
		if err := dec.ReadMsg(&buf); err != nil {
			if err == io.EOF {
				logError(buf, "shutting down log logger", err)
				c.stream.Close()
				return
			}
			dec = protoio.NewUint32DelimitedReader(c.stream, binary.BigEndian, 1e6)
		}

		// create message
		msg.Source = buf.Source
		msg.Partial = buf.Partial
		msg.Line = buf.Line
		msg.TimeNano = buf.TimeNano

		m, err := json.Marshal(msg)
		if err != nil {
			logError(msg, "error unmarshaling json log message", err)
		}
		m = append(m, '\n')

		// TODO: create a separeted gorouting for sending log messages to logstash
		if err := d.client.Write(m); err != nil {
			// TODO: fix - msg.Line log message in bytes
			logError(msg, "error sending log message to logstash", err)

			// cache log messages temporary to logfile
			if _, err := d.file.writer.Write(m); err != nil {
				logError(msg, "error writing log message", err)
			}
			if err := d.file.writer.Flush(); err != nil {
				logError(msg, "error flush log message", err)
			}
			go d.renameFile(c.info.LogPath)

			// continue // do we need this?
		}

		buf.Reset()
	}
}

func logError(msg interface{}, str string, err error) {

	logrus.WithFields(
		logrus.Fields{
			"message": msg,
			"error":   err,
		},
	).Error(str)
}

func (d *Driver) renameFile(file string) {

	// avoid starting multiple go routines
	if d.file.rename {
		return
	}

	d.file.rename = true

	timestamp := time.Now().Format(time.RFC3339Nano)

	for {
		// only rename file, if client is send messages to logstash
		if !d.client.Reconnecting() {

			// rename the file with an timestamp attached to it
			if err := os.Rename(file, file+"."+timestamp); err != nil {
				logError(file, "moving file", err)
			}

			go d.readLogFile(file + "." + timestamp)

			f, err := d.openLogFile(file)
			if err != nil {
				logError(file, "moving file", err)
				// TODO: this is bad, if no new logfile can be created
				// maybe add a retry or something
				return
			}
			d.file.fd = f

			break
		}
		time.Sleep(time.Second * 1)
	}
}

func (d *Driver) openLogFile(file string) (*os.File, error) {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, errors.Wrapf(err, "error create cache  log file: %q", f)
	}

	d.file.rename = false
	d.file.writer = bufio.NewWriter(f)

	return f, nil
}

func (d *Driver) readLogFile(file string) {

	f, err := os.Open(file)
	if err != nil {
		logError(file, "error: openning cache file", err)
	}

	reader := bufio.NewReader(f)

	// TODO: read file by chunk
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			logError(file, "error: reading cache log file", err)
			continue
		}
		if err = d.client.Write(line); err != nil {
			logError(string(line), "error: resending log message to logstash", err)
		}
	}

	logrus.WithField("file", file).Debugf("debug: removing logfile")
	if err := os.Remove(file); err != nil {
		logError(file, "error: removing logfile", err)
	}

}

func (d *Driver) StopLogging(pipe string) error {
	logrus.WithField("pipe", pipe).Debugf("Stop logging")

	d.mu.Lock()
	c, ok := d.logs[pipe]
	if ok {

		// close fifo
		c.stream.Close()
		delete(d.logs, pipe)
	}
	d.mu.Unlock()

	// close log file descriptor
	if d.file.fd != nil {
		d.file.fd.Close()
	}

	// close connection, if still open
	err := d.client.Close()
	if err != nil {
		return err
	}

	return nil
}
