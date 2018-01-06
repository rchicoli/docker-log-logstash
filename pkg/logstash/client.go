package logstash

import (
	"fmt"
	"math"
	"net"
	"time"
)

// type TransportClient interface {
// 	net.Conn
// 	Connect(timeout time.Duration) error
// 	IsConnected() bool
// }

const (
	defaultRetryWaitRate = 1.5
	defaultMaxWaitTime   = 300
)

type Config struct {
	scheme       string
	host         string
	port         string
	timeout      time.Duration
	retryWait    int
	reconnecting bool
}

type Logstash struct {
	config Config
	Conn   net.Conn
}

func NewClient(scheme, host, port string, timeout time.Duration) (*Logstash, error) {

	cfg := Config{
		scheme:       scheme,
		host:         host,
		port:         port,
		timeout:      timeout,
		reconnecting: false,
	}

	client := &Logstash{config: cfg}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("error: failed to connect")
	}
	return client, nil

}

func (l *Logstash) connect() error {

	c, err := net.DialTimeout(l.config.scheme, l.config.host+":"+l.config.port, l.config.timeout)
	if err != nil {
		return fmt.Errorf("logstash: cannot establish a connection: %v", err)
	}
	l.Conn = c

	return nil
}

func (l *Logstash) Reconnect() error {

	// it is already trying to reconnect
	if l.config.reconnecting == true {
		return nil
	}

	waitTime := 0
	l.config.reconnecting = true
	for i := 0; ; i++ {

		err := l.connect()
		if err == nil {
			return nil
		}

		// backoff wait time
		if waitTime < defaultMaxWaitTime {
			waitTime = l.config.retryWait * backoff(defaultRetryWaitRate, float64(i-1))
		}
		time.Sleep(time.Duration(waitTime) * time.Millisecond)

	}

	return nil
}

// func (l *Logstash) Write

func backoff(x, y float64) int {
	return int(math.Pow(x, y))
}
