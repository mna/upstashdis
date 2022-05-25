// Package upstashdis provides a client for the Upstash Redis REST API
// interface [1].
//
//     [1]: https://docs.upstash.com/redis/features/restapi
//
package upstashdis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
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

// Client is an Upstash Redis REST client. It is safe for concurrent use as
// long as the HTTPClient and the NewRequestFunc values are also safe for
// concurrent use (the defaults are). The fields should be set before use and
// should not be changed thereafter.
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
	// it will be used as-is, otherwise the header is set with the APIToken (or
	// the token provided to NewRequestWithToken) as Bearer value. If
	// NewRequestFunc is nil, http.NewRequest is used.
	NewRequestFunc func(method, url string, body io.Reader) (*http.Request, error)
}

// NewRequest starts a new REST API request using this client.
func (c *Client) NewRequest() *Request {
	return &Request{c: c, tok: c.APIToken}
}

// NewRequestWithToken starts a new REST API request using this client, but
// overrides the client's APIToken with the provided token. This can be useful
// for when an ACL RESTTOKEN-generated token should be used instead of the
// generic one, while still sharing the same client configuration.
func (c *Client) NewRequestWithToken(token string) *Request {
	return &Request{c: c, tok: token}
}

// A Request is started by calling Client.NewRequest. It is not safe for
// concurrent use.
type Request struct {
	c   *Client
	tok string
	req [][]interface{} // the pending requests to execute
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

// Error returns the error message.
func (e *Error) Error() string {
	return e.Message
}

func newError(msg string, ix int) *Error {
	err := &Error{Message: msg, PipelineIndex: ix}
	if kind, _, ok := strings.Cut(msg, " "); ok {
		err.Kind = kind
	}
	return err
}

// Send queues a new command to be executed when Exec is called.
func (r *Request) Send(cmd string, args ...interface{}) error {
	if cmd == "" {
		return errors.New("upstashdis: empty command")
	}

	// serialize and buffer the command
	new := make([]interface{}, len(args)+1)
	new[0] = cmd
	for i, arg := range args {
		new[i+1] = writeArg(arg, true)
	}
	r.req = append(r.req, new)
	return nil
}

// Exec executes all commands queued by calls to Send. If a single command is
// queued, a standard call is done, otherwise all commands are executed as a
// pipeline call. Note that with the Upstash Redis REST API, a pipeline call is
// not guaranteed to execute atomically.
//
// The results are unmarshaled into the provided dst values. At most len(dst)
// results are unmarshaled, and an error is returned if len(dst) > number of
// results. If any result is an error, an error is returned but any remaining
// results are unmarshaled. If more than one result is an error, only the first
// one is returned. If len(dst) < number of results, the remaining results are
// discarded. A nil dst value simply ignores the corresponding result at that
// position.
//
// The error returned will be of type *Error if it is a command that failed.
// You may inspect its fields for more information about what failed after
// obtaining its typed value with errors.As. It can also be of a different type
// if e.g. the request failed due to a network error.
func (r *Request) Exec(dst ...interface{}) error {
	res, err := r.exec()
	if err != nil {
		return err
	}

	min := len(res)
	if len(dst) < min {
		min = len(dst)
	}
	if len(dst) > len(res) {
		return errors.New("upstashdis: too many destination values")
	}

	var firstErr error
	for i := 0; i < min; i++ {
		r, d := res[i], dst[i]
		if r.Error != "" && firstErr == nil {
			firstErr = newError(r.Error, i)
			continue
		}
		if d != nil && r.Result != nil {
			if err := json.Unmarshal(r.Result, d); err != nil {
				return err
			}
		}
	}
	return firstErr
}

// ExecOne executes the provided command and unmarshals its result into dst. If
// there were commands already queued for execution, they will be executed in a
// pipeline with the provided command as last, but only the result of that last
// command will be returned - even if previous commands resulted in errors.
//
// In other words, it executes all pending commands and discards their results,
// before executing the specified command and returning its result - either as
// the returned error if it failed, or unmarshaled into dst if it succeeded. If
// dst is nil, the successful result is ignored.
func (r *Request) ExecOne(dst interface{}, cmd string, args ...interface{}) error {
	if err := r.Send(cmd, args...); err != nil {
		return err
	}

	res, err := r.exec()
	if err != nil {
		return err
	}

	// skip all but the last result
	last := res[len(res)-1]
	if last.Error != "" {
		return newError(last.Error, 0) // index is 0 because it behaves as if the rest of the pipeline did not exist
	}

	if dst == nil {
		return nil
	}
	return json.Unmarshal(last.Result, dst)
}

// ExecRaw executes all commands queued by calls to Send and returns a slice
// corresponding to the results of each command. Unlike Exec and ExecOne, it
// does not return an error if the result is an error - it stores all results
// in the returned slice, returning an error only if the request itself failed,
// e.g. due to a network error.
//
// This can be used if the caller needs to know precisely all commands that
// failed in a pipeline (as Exec returns only the first error encountered).
func (r *Request) ExecRaw() ([]*Result, error) {
	return r.exec()
}

func (r *Request) exec() ([]*Result, error) {
	var (
		body     bytes.Buffer
		pipeline bool
		err      error
	)

	// create the request (pipeline if > 1), make the call
	switch len(r.req) {
	case 0:
		return nil, errors.New("upstashdis: no command to execute")
	case 1:
		// single command
		err = json.NewEncoder(&body).Encode(r.req[0])
	default:
		// pipeline
		err = json.NewEncoder(&body).Encode(r.req)
		pipeline = true
	}
	r.req = r.req[:0]

	if err != nil {
		return nil, err
	}

	return r.makeRequest(&body, pipeline)
}

func (r *Request) makeRequest(body io.Reader, pipeline bool) ([]*Result, error) {
	httpCli := r.c.HTTPClient
	if httpCli == nil {
		httpCli = http.DefaultClient
	}

	newReq := r.c.NewRequestFunc
	if newReq == nil {
		newReq = http.NewRequest
	}

	surl := r.c.BaseURL
	if pipeline {
		purl, err := url.Parse(surl)
		if err != nil {
			return nil, err
		}
		purl.Path = path.Join(purl.Path, "pipeline")
		surl = purl.String()
	}
	req, err := newReq("POST", surl, body)
	if err != nil {
		return nil, err
	}
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+r.tok)
	}

	res, err := httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		if len(b) == 0 {
			b = []byte(res.Status)
		} else {
			// try to decode the body as JSON into a Result with an error value
			var pld Result
			if err := json.Unmarshal(b, &pld); err == nil && pld.Error != "" {
				var ix int
				if pipeline {
					ix = -1 // pipeline errors still return 200, so unrelated to a command if it is a pipeline
				}
				return nil, newError(pld.Error, ix)
			}
		}
		return nil, fmt.Errorf("[%d]: %s", res.StatusCode, string(b))
	}

	var results []*Result
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if pipeline {
		err = json.Unmarshal(raw, &results)
	} else {
		var result Result
		if err = json.Unmarshal(raw, &result); err == nil {
			results = []*Result{&result}
		}
	}
	return results, err
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
	case Argument:
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
