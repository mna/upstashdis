package upstashdis

import (
	"errors"
	"io"
	"net/http"

	"github.com/gomodule/redigo/redis"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type restResult struct {
	Error  string      `json:"error"`
	Result interface{} `json:"result"`
}

type Client struct {
	BaseURL        string
	APIToken       string
	HTTPClient     HTTPDoer
	NewRequestFunc func(method, url string, body io.Reader) (*http.Request, error)
}

func (c *Client) NewConn() redis.Conn {
	return &conn{client: c}
}

func (c *Client) Dial() (redis.Conn, error) {
	return &conn{client: c}, nil
}

var errClosed = errors.New("upstashdis: closed")

type conn struct {
	client *Client
	err    error
}

func (c *conn) Close() error {
	if c.err != nil {
		return c.err
	}
	c.err = errClosed
	return nil
}

func (c *conn) Err() error {
	return c.err
}

func (c *conn) Do(cmd string, args ...interface{}) (interface{}, error) {
	if err := c.Send(cmd, args...); err != nil {
		return nil, err
	}
	if err := c.Flush(); err != nil {
		return nil, err
	}
	return c.Receive()
}

func (c *conn) Send(cmd string, args ...interface{}) error {
	// serialize and buffer the command
	return nil
}

func (c *conn) Flush() error {
	// create the request (pipeline if > 1), make the call
	return nil
}

func (c *conn) Receive() (interface{}, error) {
	// wait for the response, if a pipeline return the next response
	return nil, nil
}
