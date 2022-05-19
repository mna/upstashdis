// Command upstash-redis-rest-server runs a web server that serves an Upstash
// Redis REST API-compatible server (see [1]). It communicates with a running
// Redis instance to execute the commands, and translates the requests and
// responses to the REST API format.
//
//     [1]: https://docs.upstash.com/redis/features/restapi
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mna/mainer"
)

const binName = "upstash-redis-rest-server"

var (
	shortUsage = fmt.Sprintf(`
usage: %s [<option>...] <command> [<path>...]
Run '%[1]s --help' for details.
`, binName)

	longUsage = fmt.Sprintf(`usage: %s --redis-addr <ADDR>
       %[1]s -h|--help

Run a web server that serves an Upstash-compatible Redis REST API and
connects to a running Redis instance to execute commands.

Valid flag options are:
       -a --addr ADDR            Address for the web server to listen on.
       -h --help                 Show this help.
       -r --redis-addr ADDR      Use the Redis instance running at this
                                 ADDR to execute commands.

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
	Addr      string `flag:"a,addr"`
	RedisAddr string `flag:"r,redis-addr"`
	Help      bool   `flag:"h,help"`

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
	var p mainer.Parser
	if err := p.Parse(args, c); err != nil {
		fmt.Fprintf(stdio.Stderr, "invalid arguments: %s\n%s", err, shortUsage)
		return mainer.InvalidArgs
	}

	if c.Help {
		fmt.Fprint(stdio.Stdout, longUsage)
		return mainer.Success
	}

	// TODO: start the web server

	return mainer.Success
}

func main() {
	var c cmd
	os.Exit(int(c.Main(os.Args, mainer.CurrentStdio())))
}
