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

	"github.com/docker/docker/api/types/backend"
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

type LoggerInfo struct {
	Config              map[string]string `json:"config,omitempty"`
	ContainerID         string            `json:"containerID"`
	ContainerName       string            `json:"containerName"`
	ContainerEntrypoint string            `json:"containerEntrypoint,omitempty"`
	ContainerArgs       []string          `json:"containerArgs,omitempty"`
	ContainerImageID    string            `json:"containerImageID,omitempty"`
	ContainerImageName  string            `json:"containerImageName,omitempty"`
	ContainerCreated    time.Time         `json:"containerCreated"`
	ContainerEnv        []string          `json:"containerEnv,omitempty"`
	ContainerLabels     map[string]string `json:"containerLabels,omitempty"`
	LogPath             string            `json:"logPath,omitempty"`
	DaemonName          string            `json:"daemonName,omitempty"`
}

type Driver struct {
	mu     sync.Mutex
	logs   map[string]*logPair
	logger logger.Logger

	client *logstash.Client
	// conn net.Conn
}

type logPair struct {
	file   *os.File
	stream io.ReadCloser
	info   logger.Info
}

type LogMessage struct {
	// logdriver.LogEntry
	Line   []byte `json:"-"`
	Source string `json:"source"`
	// Timestamp time.Time         `json:"@timestamp"`
	Attrs []backend.LogAttr `json:"attr,omitempty"`
	// Partial   bool              `json:"partial"`

	// Err is an error associated with a message. Completeness of a message
	// with Err is not expected, tho it may be partially complete (fields may
	// be missing, gibberish, or nil)
	Err error `json:"err,omitempty"`

	LoggerInfo
	LogLine string `json:"message"`
}

func NewDriver() *Driver {

	return &Driver{
		logs: make(map[string]*logPair),
	}
}

func (d *Driver) StartLogging(file string, info logger.Info) error {

	// get config from log-opt and validate it
	cfg := defaultLogOpt()
	if err := cfg.validateLogOpt(info.Config); err != nil {
		return errors.Wrapf(err, "error: logstash-options: %q", err)
	}
	// logrus.WithField("id", info.ContainerID).Debugf("log-opt: %v", cfg)

	d.mu.Lock()
	if _, exists := d.logs[file]; exists {
		d.mu.Unlock()
		return fmt.Errorf("logger for %q already exists", file)
	}
	d.mu.Unlock()

	// cache messages to the filesystem, if the target service is not responding
	if info.LogPath == "" {
		info.LogPath = filepath.Join("/var/log/docker", info.ContainerID+".log")
	}
	if err := os.MkdirAll(filepath.Dir(info.LogPath), 0755); err != nil {
		return errors.Wrap(err, "error setting up logger dir")
	}
	l, err := os.OpenFile(info.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return errors.Wrapf(err, "error create cache  log file: %q", l)
	}

	ctx := context.Background()

	logrus.WithField("id", info.ContainerID).WithField("file", file).WithField("logpath", info.LogPath).Debugf("Start logging")
	f, err := fifo.OpenFifo(ctx, file, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", file)
	}

	d.mu.Lock()
	lf := &logPair{
		file:   l,
		stream: f,
		info:   info,
	}
	d.logs[file] = lf
	d.mu.Unlock()

	d.client, err = logstash.NewClient(cfg.scheme, cfg.host+":"+cfg.port, cfg.timeout)
	if err != nil {
		return fmt.Errorf("logstash: cannot establish a connection: %v", err)
	}

	// TODO: add context
	go d.consumeLog(lf)
	return nil
}

func (d *Driver) consumeLog(lf *logPair) {

	dec := protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
	defer dec.Close()

	var buf logdriver.LogEntry
	var msg LogMessage

	writer := bufio.NewWriter(lf.file)

	for {
		if err := dec.ReadMsg(&buf); err != nil {
			if err == io.EOF {
				logrus.WithField("id", lf.info.ContainerID).WithError(err).Debug("shutting down log logger")
				lf.stream.Close()
				return
			}
			dec = protoio.NewUint32DelimitedReader(lf.stream, binary.BigEndian, 1e6)
		}

		// create message
		// msg.Timestamp = time.Unix(0, buf.TimeNano)
		msg.Source = buf.Source
		// msg.Partial = buf.Partial
		msg.LogLine = strings.TrimSpace(string(buf.Line))

		// msg.Config = lf.info.Config
		msg.ContainerID = lf.info.ID()
		msg.ContainerName = lf.info.Name()
		// msg.ContainerEntrypoint = lf.info.ContainerEntrypoint
		// msg.ContainerArgs = lf.info.ContainerArgs
		// msg.ContainerImageID = lf.info.ContainerImageID
		msg.ContainerImageName = lf.info.ContainerImageName
		msg.ContainerCreated = lf.info.ContainerCreated
		// msg.ContainerEnv = lf.info.ContainerEnv
		// msg.ContainerLabels = lf.info.ContainerLabels
		// msg.LogPath = lf.info.LogPath
		// msg.DaemonName = lf.info.DaemonName

		m, err := json.Marshal(msg)
		if err != nil {
			logrus.WithField("id", lf.info.ContainerID).
				WithError(err).
				WithField("message", msg).
				Error("error unmarshaling json log message")
		}
		m = append(m, '\n')

		if _, err := d.conn.Write(m); err != nil {
			logrus.WithField("id", lf.info.ContainerID).
				WithError(err).
				WithField("message", msg).
				Error("error sending log message to logstash")

			//TODO: reconnect and retry

			// if retry not possible cache to logfile
			if _, err := writer.Write(m); err != nil {
				logrus.WithField("id", lf.info.ContainerID).WithError(err).WithField("message", msg).Error("error writing log message")
			}
			if err := writer.Flush(); err != nil {
				logrus.WithField("id", lf.info.ContainerID).
					WithError(err).
					WithField("message", msg).
					Error("error flush log message")
			}
			// continue // do we need this?
		}

		buf.Reset()
	}
}

func (d *Driver) StopLogging(file string) error {
	logrus.WithField("file", file).Debugf("Stop logging")

	d.mu.Lock()
	lf, ok := d.logs[file]
	if ok {
		// close log file
		d.logs[file].file.Close()

		// close fifo
		lf.stream.Close()
		delete(d.logs, file)
	}
	d.mu.Unlock()

	// close connection, if still open
	if d.conn != nil {
		err := d.conn.Close()
		if err != nil {
			return err
		}
	}

	//TODO: log-opt delete rootfs

	return nil
}
