package docker

import (
	"fmt"
	"net"
	"net/url"
)

type LogOpt struct {
	url string
}

func defaultLogOpt() *LogOpt {
	return &LogOpt{}
}

func parseAddress(address string) error {
	if address == "" {
		return fmt.Errorf("error parsing logstash url")
	}

	url, err := url.Parse(address)
	if err != nil {
		return err
	}

	if url.Scheme != "tcp" {
		return fmt.Errorf("logstash: endpoint accepts only http at the moment")
	}

	_, _, err = net.SplitHostPort(url.Host)
	if err != nil {
		return fmt.Errorf("logstash: please provide logstash-url as host:port")
	}

	return nil
}

// ValidateLogOpt looks for es specific log option es-address.
func (c *LogOpt) validateLogOpt(cfg map[string]string) error {
	for key, v := range cfg {
		switch key {
		case "logstash-url":
			if err := parseAddress("tcp://" + v); err != nil {
				return err
			}
			c.url = v
		default:
			return fmt.Errorf("unknown log opt %q for logstash log Driver", key)
		}
	}

	return nil
}
