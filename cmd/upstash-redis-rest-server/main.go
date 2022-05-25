// Command upstash-redis-rest-server runs a web server that serves an Upstash
// Redis REST API-compatible server (see [1]). It communicates with a running
// Redis instance to execute the commands, and translates the requests and
// responses to the REST API format.
//
//     [1]: https://docs.upstash.com/redis/features/restapi
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/mna/mainer"
	"github.com/mna/upstashdis/restserver"
)

const binName = "upstash-redis-rest-server"

var (
	shortUsage = fmt.Sprintf(`
usage: %s --addr <ADDR> --redis-addr <ADDR> [--api-token <TOKEN>]
Run '%[1]s --help' for details.
`, binName)

	longUsage = fmt.Sprintf(`usage: %s --addr <ADDR> --redis-addr <ADDR> [--api-token <TOKEN>]
       %[1]s --help

Run a web server that serves an Upstash-compatible Redis REST API and
connects to a running Redis instance to execute commands.

Valid flag options are:
       -a --addr ADDR            Address for the web server to listen on.
       -h --help                 Show this help.
       -r --redis-addr ADDR      Use the Redis instance running at this
                                 ADDR to execute commands.
       -t --api-token TOKEN      API token to accept as authorized. Can
                                 also be set via the environment variable
                                 UPSTASH_REDIS_REST_SERVER_API_TOKEN.

The redis instance should be version 6 and above for better
compatibility.

As a special case, 'memory' can be used as --redis-addr and a miniredis
instance will be used. Note that not all commands are supported by
miniredis - in particular, ACL commands are not supported. See the
miniredis repository and documentation for more information:
       https://github.com/alicebob/miniredis.

More information on the upstashdis repository:
       https://github.com/mna/upstashdis
`, binName)
)

type cmd struct {
	Addr      string `flag:"a,addr" ignored:"true"`
	APIToken  string `flag:"t,api-token" envconfig:"api_token"`
	RedisAddr string `flag:"r,redis-addr" ignored:"true"`
	Help      bool   `flag:"h,help" ignored:"true"`

	args []string
}

func (c *cmd) SetArgs(args []string) {
	c.args = args
}

func (c *cmd) Validate() error {
	if c.Help {
		return nil
	}

	if len(c.args) > 0 {
		return errors.New("unexpected arguments provided")
	}
	if c.Addr == "" {
		return errors.New("no --addr provided")
	}
	if c.RedisAddr == "" {
		return errors.New("no --redis-addr provided")
	}
	return nil
}

func (c *cmd) Main(args []string, stdio mainer.Stdio) mainer.ExitCode {
	p := mainer.Parser{
		EnvVars:   true,
		EnvPrefix: strings.ReplaceAll(binName, "-", "_"),
	}
	if err := p.Parse(args, c); err != nil {
		fmt.Fprintf(stdio.Stderr, "invalid arguments: %s\n%s", err, shortUsage)
		return mainer.InvalidArgs
	}

	if c.Help {
		fmt.Fprint(stdio.Stdout, longUsage)
		return mainer.Success
	}

	// start miniredis is requested
	raddr := c.RedisAddr
	if raddr == "memory" {
		miniRed, err := miniredis.Run()
		if err != nil {
			fmt.Fprintf(stdio.Stderr, "failed to start miniredis: %s\n", err)
			return mainer.Failure
		}
		defer miniRed.Close()
		raddr = miniRed.Addr()
	}

	// configure the REST server
	pool := makePool(raddr)
	usrv := &restserver.Server{
		APIToken: c.APIToken,
		GetConnFunc: func(ctx context.Context) restserver.Conn {
			return pool.Get()
		},
	}

	// start the web server
	log.Printf("listening on %s...", c.Addr)
	if err := http.ListenAndServe(c.Addr, usrv); err != nil {
		fmt.Fprintf(stdio.Stderr, "web server error: %s\n", err)
		return mainer.Failure
	}
	return mainer.Success
}

func makePool(addr string) *redis.Pool {
	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", addr)
		},
	}
}

func main() {
	var c cmd
	os.Exit(int(c.Main(os.Args, mainer.CurrentStdio())))
}
