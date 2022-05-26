package restserver_test

import (
	"context"
	"net/http/httptest"

	"github.com/gomodule/redigo/redis"
	"github.com/mna/upstashdis/restserver"
)

func ExampleServer_pool() {
	// create the redigo pool, connecting to a running Redis instance
	pool := redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", "localhost:6379")
		},
	}
	defer pool.Close()

	// configure the REST server to get connections from that pool
	server := &restserver.Server{
		APIToken: "<token>",
		GetConnFunc: func(ctx context.Context) restserver.Conn {
			return pool.Get()
		},
	}

	// create a test web server to serve the REST server.
	httpsrv := httptest.NewServer(server)
	defer httpsrv.Close()

	// the REST server is available at httpsrv.URL
}
