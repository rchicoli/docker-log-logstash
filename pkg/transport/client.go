package transport

import (
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/felixge/tcpkeepalive"
)

// type TransportClient interface {
// 	// net.Conn

// 	Write(payload []byte) error
// 	Close() error
// 	Reconnecting() bool

// 	isConnected() bool
// 	reconnect()
// }

const (
	defaultRetryWaitRate = 1.5
	defaultMaxWaitTime   = 300 * 1000
)

var (
	zeroByte = make([]byte, 1, 1)
)

type Config struct {
	scheme    string
	host      string
	port      string
	timeout   time.Duration
	retryWait int
	reconnect bool
}

type Client struct {
	config Config
	conn   net.Conn
	mu     sync.Mutex
}

func NewClient(scheme, host, port string, timeout time.Duration) (*Client, error) {

	cfg := Config{
		scheme:    scheme,
		host:      host,
		port:      port,
		timeout:   timeout,
		reconnect: false,
	}
	client := &Client{config: cfg}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("error: failed to connect")
	}
	return client, nil

}

func (c *Client) connect() error {

	conn, err := net.DialTimeout(c.config.scheme, fmt.Sprintf("%s:%s", c.config.host, c.config.port), c.config.timeout)
	if err != nil {
		return fmt.Errorf("logstash: cannot establish a connection: %v", err)
	}

	// TODO: add logstash-idle-time logstash-healthcheck-max-count-failed logstash-healthcheck-interval
	err = tcpkeepalive.SetKeepAlive(conn, 5*time.Second, 5, 5*time.Second)
	if err != nil {
		return fmt.Errorf("error: keepalive: %v", err)
	}

	c.conn = conn
	c.config.reconnect = false

	return nil
}

func (c *Client) Write(payload []byte) error {

	if !c.isConnected() {
		go c.reconnect()
		return fmt.Errorf("error: connection has been closed")
	}

	b, err := c.conn.Write(payload)
	if err != nil {
		go c.reconnect()
		return fmt.Errorf("logstash: cannot send payload: %v", err)
	}
	if b != len(payload) {
		return fmt.Errorf("logstash: send error")
	}

	return nil
}

func (c *Client) reconnect() {

	c.mu.Lock()
	if c.Reconnecting() {
		return
	}
	c.config.reconnect = true
	c.mu.Unlock()

	c.Close()

	waitTime := 0
	for i := 0; ; i++ {

		err := c.connect()
		if err == nil {
			return
		}

		// backoff wait time
		if waitTime < defaultMaxWaitTime {
			waitTime = c.config.retryWait * exponential(defaultRetryWaitRate, float64(i-1))
		}
		time.Sleep(time.Duration(waitTime) * time.Millisecond)

	}

}

func (c *Client) Close() error {

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// isConnected checks if the connection has been closed
// on the server side.
func (c *Client) isConnected() bool {

	c.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, err := c.conn.Read(zeroByte); err == io.EOF {
		return false
	}
	return true
}

func exponential(x, y float64) int {
	return int(math.Pow(x, y))
}

func (c *Client) Reconnecting() bool {
	return c.config.reconnect
}
