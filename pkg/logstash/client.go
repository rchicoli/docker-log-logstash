package logstash

import (
	"fmt"
	"net"
	"time"
)

type TransportClient interface {
	net.Conn
	Connect(timeout time.Duration) error
	IsConnected() bool
}

type Client struct {
	conn net.Conn
}

func NewClient(scheme, host, port string, timeout time.Duration) (*Client, error) {
	c, err := net.DialTimeout(scheme, host+":"+port, timeout)
	if err != nil {
		return nil, fmt.Errorf("logstash: cannot establish a connection: %v", err)
	}

	return &Logstash{
		conn: c,
	}, nil
}

func (l *Logstash) connect() error {

	return nil
}

func (l *Logstash) reconnect() error {

	return nil
}
