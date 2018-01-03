package docker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

	// client *logstash.Client
	conn net.Conn
}

type logPair struct {
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
	d.mu.Lock()
	if _, exists := d.logs[file]; exists {
		d.mu.Unlock()
		return fmt.Errorf("logger for %q already exists", file)
	}
	d.mu.Unlock()

	ctx := context.Background()

	logrus.WithField("id", info.ContainerID).WithField("file", file).WithField("logpath", info.LogPath).Debugf("Start logging")
	f, err := fifo.OpenFifo(ctx, file, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", file)
	}

	d.mu.Lock()
	lf := &logPair{
		stream: f,
		info:   info,
	}
	d.logs[file] = lf
	d.mu.Unlock()

	cfg := defaultLogOpt()
	if err := cfg.validateLogOpt(info.Config); err != nil {
		return errors.Wrapf(err, "error: logstash-options: %q", err)
	}
	logrus.WithField("id", info.ContainerID).Debugf("log-opt: %v", cfg)

	d.conn, err = net.Dial("tcp", cfg.url)
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
		if _, err := d.conn.Write(m); err != nil {
			logrus.WithField("id", lf.info.ContainerID).
				WithError(err).
				WithField("message", msg).
				Error("error sending log message")
			continue
		}
		_, _ = d.conn.Write([]byte("\n"))

		buf.Reset()
	}
}

func (d *Driver) StopLogging(file string) error {
	logrus.WithField("file", file).Debugf("Stop logging")
	d.mu.Lock()
	lf, ok := d.logs[file]
	if ok {
		lf.stream.Close()
		delete(d.logs, file)
	}
	d.mu.Unlock()
	return nil
}
