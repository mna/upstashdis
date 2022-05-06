// Package upstashdis provides a redigo-compatible connection for the Upstash
// Redis REST API interface.
package upstashdis

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gomodule/redigo/redis"
)

// HTTPDoer defines the method required for an HTTP client. The
// *net/http/Client standard library type satisfies this interface.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type restResult struct {
	Error  string      `json:"error"`
	Result interface{} `json:"result"`
}

// Client is an Upstash Redis REST client.
type Client struct {
	// BaseURL is the base URL of the Upstash Redis REST API.
	BaseURL string
	// APIToken is the Upstash Redis REST API token used for authentication.
	APIToken string
	// HTTPClient is the HTTP client to use to make the REST API requests. If
	// nil, http.DefaultClient is used.
	HTTPClient HTTPDoer
	// NewRequestFunc is the function used to create the HTTP Request for each
	// REST API request. If nil, http.NewRequest is used.
	NewRequestFunc func(method, url string, body io.Reader) (*http.Request, error)
}

// NewConn creates a redigo-compatible Redis connection that uses the Upstash
// client internally to execute commands using the REST API. Since this is a
// connection-less mode of execution, NewConn cannot fail and always returns a
// valid connection instantly. Hence, there is no point in using a connection
// pool with those "connections" - there is no connection overhead, and the
// small memory allocation of a connection is unlikely to have much of an
// effect.
//
// The concurrency characteristics of the returned connection are different
// than the ones for standard redigo connections [1]. The connection is not
// safe to use concurrently in any case, for any of its methods.
//
// While this may seem like an important restriction, in practice it is not -
// the Upstash Redis REST API does not support subscribing and listening to
// pub-sub channels [2], the main use-case for concurrent Send-Flush and Receive
// calls.
//
//     [1]: https://pkg.go.dev/github.com/gomodule/redigo/redis#hdr-Concurrency
//     [2]: https://docs.upstash.com/redis/features/restapi#rest---redis-api-compatibility
//
func (c *Client) NewConn() redis.Conn {
	return &conn{client: c}
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
// Send, if any.
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
	if cmd == "" && len(args) == 0 {
		return nil
	}

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

// adjusted from redigo's internal helper function.
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
