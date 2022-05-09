// Package upstashdis provides a client for the Upstash Redis REST API
// interface.
package upstashdis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gomodule/redigo/redis"
)

// Argument allows arbitrary values to encode themselves as a valid argument
// value for a Redis command. This is compatible with redigo's redis.Argument
// interface.
type Argument interface {
	// RedisArg returns a value to be encoded as a bulk string.
	// Implementations should typically return a []byte or string.
	RedisArg() interface{}
}

// HTTPDoer defines the method required for an HTTP client. The
// *net/http/Client standard library type satisfies this interface.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Result is the struct used to unmarshal raw results for a command execution.
// The Error string is unmarshaled, while the successful result is stored as
// a json.RawMessage until it gets unmarshaled into a destination value in a
// call to Exec or ExecOne, or manually by the caller after ExecRaw.
type Result struct {
	Error  string          `json:"error"`
	Result json.RawMessage `json:"result"`
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
	// REST API request. If the returned request has an Authorization header set,
	// it will be used as-is, otherwise the header is set with the APIToken as
	// Bearer value. If NewRequestFunc is nil, http.NewRequest is used.
	NewRequestFunc func(method, url string, body io.Reader) (*http.Request, error)

	// the pending requests to execute
	req [][]interface{}
}

// Error represents an error returned by Redis.
type Error struct {
	// Message is the full error message, including its Kind, if present.
	Message string
	// Kind is the error kind as used by Redis as a convention. This is the first
	// word in the error message, and all in uppercase. It may be empty.
	Kind string
	// PipelineIndex is the index of the command that returned this error in the
	// pipeline, 0 if the command was executed without pipeline, and -1 if the
	// error did not originate from a command execution.
	PipelineIndex int
}

// Send queues a new command to be executed when Exec is called.
func (c *Client) Send(cmd string, args ...interface{}) error {
	if cmd == "" {
		return errors.New("upstashdis: empty command")
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

// Exec executes all commands queued by calls to Send. If a single command is
// queued, a standard call is done, otherwise all commands are executed as a
// pipeline call. Note that with the Upstash Redis REST API, a pipeline call is
// not guaranteed to execute atomically.
//
// The results are unmarshalled into the provided dst values. At most len(dst)
// results are unmarshaled, and an error is returned if len(dst) > number of
// results. If any result is an error, an error is returned but any remaining
// results are unmashalled. If more than one result is an error, only the first
// one is returned. If len(dst) < number of results, the remaining results are
// discarded.
//
// The error returned might be of type *Error. You may inspect its fields for
// more information about what failed after obtaining its typed value with
// errors.As. It can also be of a different type if e.g. the request failed due
// to a network error.
func (c *Client) Exec(dst ...interface{}) error {
	return nil
}

// ExecOne executes the provided command and unmarshals its result into dst. If
// there were commands already queued for execution, they will be executed in a
// pipeline with the provided command as last, but only the result of that last
// command will be returned - even if previous commands resulted in errors.
//
// In other words, it executes all pending commands and discards their results,
// before executing the specified command and returning its result - either as
// the returned error if it failed, or unmarshaled into dst if it succeeded.
func (c *Client) ExecOne(dst interface{}, cmd string, args ...interface{}) error {
	return nil
}

// ExecRaw executes all commands queued by calls to Send and returns a slice
// corresponding to the results of each command. Unlike Exec and ExecOne, it
// does not return an error if the result is an error - it stores all results
// in the returned slice, returning an error only if the request itself failed,
// e.g. due to a network error.
//
// This can be used if the caller needs to know precisely all commands that
// failed in a pipeline (as Exec returns only the first error encountered).
func (c *Client) ExecRaw() ([]*Result, error) {
	return nil, nil
}

/*
// Flush sends any pending commands, returning an error if it fails to marshal
// the commands or send the request - including if the server returns a non-200
// status code.
func (c *conn) Flush() error {
	var (
		body     bytes.Buffer
		pipeline bool
		err      error
	)

	// create the request (pipeline if > 1), make the call
	switch len(c.req) {
	case 0:
		// no-op
		return nil
	case 1:
		// single command
		err = json.NewEncoder(&body).Encode(c.req[0])
	default:
		// pipeline
		err = json.NewEncoder(&body).Encode(c.req)
		pipeline = true
	}
	c.req = c.req[:0]

	if err != nil {
		return err
	}

	return c.makeRequest(&body, pipeline)
}

func (c *conn) makeRequest(body io.Reader, pipeline bool) error {
	httpCli := c.client.HTTPClient
	if httpCli == nil {
		httpCli = http.DefaultClient
	}

	newReq := c.client.NewRequestFunc
	if newReq == nil {
		newReq = http.NewRequest
	}

	surl := c.client.BaseURL
	if pipeline {
		purl, err := url.Parse(surl)
		if err != nil {
			return err
		}
		purl.Path = path.Join(purl.Path, "pipeline")
		surl = purl.String()
	}
	req, err := newReq("POST", surl, body)
	if err != nil {
		return err
	}
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+c.client.APIToken)
	}

	res, err := httpCli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		if len(b) == 0 {
			b = []byte(res.Status)
		} else {
			// try to decode the body as JSON into a restResult with an error value
			var pld restResult
			if err := json.Unmarshal(b, &pld); err == nil && pld.Error != "" {
				return redis.Error(pld.Error)
			}
		}
		return fmt.Errorf("[%d]: %s", res.StatusCode, string(b))
	}

	var results []restResult
	if pipeline {
		if err := json.NewDecoder(res.Body).Decode(&results); err != nil {
			return err
		}
	} else {
		var result restResult
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			return err
		}
		results = []restResult{result}
	}

	// adjust results types - Redis returns floats as strings, but Go will
	// unmarshal integers as float64. Convert them to integers.
	c.res = results
	return nil
}
*/

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
