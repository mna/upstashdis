package upstashdis

import (
	"bytes"
	"errors"
	"fmt"
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
	client  *Client
	req     [][]interface{}
	res     []restResult
	httpRes *http.Response
	err     error
}

// Close releases the connection, making it unusable for future requests.
// Subsequent calls to Close will return an error indicating that it is closed.
func (c *conn) Close() error {
	if c.err != nil {
		return c.err
	}
	c.err = errClosed
	return nil
}

// Err returns the error that terminated the connection.
func (c *conn) Err() error {
	return c.err
}

// Do executes a command, waits for its result and returns it. If "" is
// provided as cmd, it just executes the commands already buffered via calls to
// Send.
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
	new := make([]interface{}, len(args)+1)
	new[0] = cmd
	for i, arg := range args {
		new[i+1] = writeArg(arg, true)
	}
	c.req = append(c.req, new)
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

func writeArg(arg interface{}, argumentTypeOK bool) interface{} {
	switch arg := arg.(type) {
	case string:
		return arg
	case []byte:
		return string(arg)
	case int:
		return int64(arg)
	case int64:
		return arg
	case float64:
		return arg
	case bool:
		if arg {
			return "1"
		} else {
			return "0"
		}
	case nil:
		return ""
	case redis.Argument:
		if argumentTypeOK {
			return writeArg(arg.RedisArg(), false)
		}
		// See comment in default clause below.
		var buf bytes.Buffer
		fmt.Fprint(&buf, arg)
		return buf.String()
	default:
		// This default clause is intended to handle builtin numeric types.
		// The function should return an error for other types, but this is not
		// done for compatibility with previous versions of the package.
		var buf bytes.Buffer
		fmt.Fprint(&buf, arg)
		return buf.String()
	}
}
