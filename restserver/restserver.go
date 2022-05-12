package restserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Conn defines the methods required for a redis connection. It is a subset
// of the popular redigo Conn interface.
type Conn interface {
	// Close closes the connection.
	Close() error

	// Do sends a command to the server and returns the received reply.
	// This function will use the timeout which was set when the connection is created
	Do(commandName string, args ...interface{}) (reply interface{}, err error)
}

type Server struct {
	APIToken    string
	GetConnFunc func(context.Context) Conn
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(requestToken(r)) {
		reply(w, errorResult{"Unauthorized"}, http.StatusUnauthorized)
		return
	}

	// only GET or POST methods are allowed
	if r.Method != "GET" && r.Method != "POST" {
		reply(w, nil, http.StatusMethodNotAllowed) // no body returned in that case
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		reply(w, errorResult{err.Error()}, http.StatusInternalServerError)
		return
	}

	conn := s.GetConnFunc(r.Context())
	defer conn.Close()

	// both GET and POST are supported regardless of how data is sent (path,
	// body, query string).
	switch path := r.URL.Path; strings.TrimSuffix(path, "/") {
	case "":
		var args []interface{}

		// a full single command in the body (a single array)
		if err := json.Unmarshal(body, &args); err != nil {
			reply(w, errorResult{"ERR failed to parse command"}, http.StatusBadRequest)
			return
		}
		if len(args) == 0 {
			reply(w, errorResult{"ERR empty command"}, http.StatusBadRequest)
			return
		}

		cmd, _ := args[0].(string)
		res, err := conn.Do(cmd, args[1:]...)
		if err != nil {
			reply(w, errorResult{err.Error()}, http.StatusBadRequest)
			return
		}
		reply(w, successResult{res}, http.StatusOK)
		return

	case "/pipeline":
		var cmds [][]interface{}

		// multiple full commands in the body (an array of arrays)
		if err := json.Unmarshal(body, &cmds); err != nil {
			reply(w, errorResult{"ERR failed to parse pipeline request"}, http.StatusBadRequest)
			return
		}
		// TODO: execute pipeline one at a time, no atomic guarantee in upstash pipeline

	default:
		// the single command is made of the path, optional body and optional query
		segments := strings.Split(path, "/")
		// remove the first segment which will always be empty
		segments = segments[1:]

		// if there's a body, it comes after the path segments
		if len(body) > 0 {
			segments = append(segments, string(body))
		}

		// if there are query values, they come last
		qparts := strings.Split(r.URL.RawQuery, "&")
		for _, qpart := range qparts {
			// if the query key has a value, then it becomes 2 redis arguments, e.g.
			// EX=100.
			kv := strings.SplitN(qpart, "=", 2)
			segments = append(segments, kv...)
		}

		args := make([]interface{}, len(segments)-1)
		for i, v := range segments[1:] {
			args[i] = v
		}
		res, err := conn.Do(segments[0], args...)
		if err != nil {
			reply(w, errorResult{err.Error()}, http.StatusBadRequest)
			return
		}
		reply(w, successResult{res}, http.StatusOK)
		return
	}
}

type errorResult struct {
	Error string `json:"error"`
}

type successResult struct {
	Result interface{} `json:"result"`
}

func reply(w http.ResponseWriter, v interface{}, status int) {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func requestToken(r *http.Request) string {
	// token is either in Authorization header or _token query string
	tok := r.URL.Query().Get("_token")
	if tok == "" {
		tok = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	return tok
}

func (s *Server) authenticate(tok string) bool {
	if tok == s.APIToken {
		return true
	}
	// else look for ACL RESTTOKEN authentication...
	return false
}
