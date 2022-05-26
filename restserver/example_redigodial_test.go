package restserver_test

import (
	"context"
	"net/http/httptest"

	"github.com/gomodule/redigo/redis"
	"github.com/mna/upstashdis/restserver"
)

// create a failed connection type for when Dial fails to get a connection.
type failedConn struct {
	err error
}

// implement the restserver.Conn interface, returning the error for each call.
func (f failedConn) Do(_ string, _ ...interface{}) (interface{}, error) {
	return nil, f.err
}

func (f failedConn) Close() error {
	return f.err
}

func ExampleServer_dial() {
	// configure the REST server to get connections by dialing to a running Redis
	// instance.
	server := &restserver.Server{
		APIToken: "<token>",
		GetConnFunc: func(ctx context.Context) restserver.Conn {
			conn, err := redis.DialContext(ctx, "tcp", "localhost:6379")
			if err != nil {
				return failedConn{err: err}
			}
			return conn
		},
	}

	// create a test web server to serve the REST server.
	httpsrv := httptest.NewServer(server)
	defer httpsrv.Close()

	// the REST server is available at httpsrv.URL
}
