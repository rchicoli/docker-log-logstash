package docker

import (
	"fmt"
	"net"
	"net/url"
)

type LogOpt struct {
	scheme string
	host   string
	port   string
}

func defaultLogOpt() *LogOpt {
	return &LogOpt{}
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
		default:
			return fmt.Errorf("unknown log opt %q for logstash log Driver", key)
		}
	}

	return nil
}
