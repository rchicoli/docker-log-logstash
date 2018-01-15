package docker

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/daemon/logger"
)

type LogOpt struct {
	scheme  string
	host    string
	port    string
	timeout time.Duration
	fields  string
}

func defaultLogOpt() *LogOpt {
	return &LogOpt{
		timeout: time.Millisecond * 1000,
		fields:  "containerID,containerName,containerImageName,containerCreated",
	}
}

func parseAddress(address string) (string, string, string, error) {
	if address == "" {
		return "", "", "", fmt.Errorf("error parsing logstash url")
	}

	url, err := url.Parse(address)
	if err != nil {
		return "", "", "", err
	}

	switch url.Scheme {
	case "tcp":
	case "udp":
	case "socker":
	default:
		return "", "", "", fmt.Errorf("logstash: endpoint accepts only http at the moment")

	}

	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		return "", "", "", fmt.Errorf("logstash: provide logstash-url as scheme://host:port")
	}

	return url.Scheme, host, port, nil
}

// ValidateLogOpt looks for es specific log option es-address.
func (c *LogOpt) validateLogOpt(cfg map[string]string) error {
	for key, v := range cfg {
		switch key {
		case "logstash-url":
			scheme, host, port, err := parseAddress(v)
			if err != nil {
				return err
			}
			c.scheme = scheme
			c.host = host
			c.port = port
		case "logstash-timeout":
			t, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("error: parsing logstash-timeout: %v", v)
			}
			c.timeout = time.Millisecond * time.Duration(t)
		case "logstash-fields":
			for _, v := range strings.Split(v, ",") {
				switch v {
				case "config":
				case "containerID":
				case "containerName":
				case "containerEntrypoint":
				case "containerArgs":
				case "containerImageID":
				case "containerImageName":
				case "containerCreated":
				case "containerEnv":
				case "containerLabels":
				case "logPath":
				case "daemonName":
				default:
					return fmt.Errorf("logstash-fields: invalid parameter %s", v)
				}
			}
			c.fields = v
		default:
			return fmt.Errorf("logstash-opt: unknown option %q for logstash log driver", key)
		}
	}

	return nil
}

func getLostashFields(fields string, info logger.Info) LogMessage {
	var l LogMessage
	for _, v := range strings.Split(fields, ",") {
		switch v {
		case "config":
			l.Config = info.Config
		case "containerID":
			l.ContainerID = info.ID()
		case "containerName":
			l.ContainerName = info.Name()
		case "containerEntrypoint":
			l.ContainerEntrypoint = info.ContainerEntrypoint
		case "containerArgs":
			l.ContainerArgs = info.ContainerArgs
		case "containerImageID":
			l.ContainerImageID = info.ContainerImageID
		case "containerImageName":
			l.ContainerImageName = info.ContainerImageName
		case "containerCreated":
			l.ContainerCreated = info.ContainerCreated
		case "containerEnv":
			l.ContainerEnv = info.ContainerEnv
		case "containerLabels":
			l.ContainerLabels = info.ContainerLabels
		case "logPath":
			l.LogPath = info.LogPath
		case "daemonName":
			l.DaemonName = info.DaemonName
		default:
		}
	}
	// TODO: omityempty does not work for type time
	l.ContainerCreated = info.ContainerCreated
	return l
}
