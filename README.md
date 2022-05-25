[![Go Reference](https://pkg.go.dev/badge/github.com/mna/upstashdis.svg)](https://pkg.go.dev/github.com/mna/upstashdis)
[![Build Status](https://github.com/mna/upstashdis/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/mna/upstashdis/actions)

# upstashdis

Package `upstashdis` provides a Go client for the [Upstash Redis REST API](https://docs.upstash.com/redis/features/restapi) interface. Note that this package is *not* affiliated with Upstash. It also provides a `restserver` Go package and an `upstash-redis-rest-server` executable command to run a local web server that serves an Upstash-compatible REST API in front of an actual Redis database instance, for testing purposes.

## Installation

Go1.18+ is required. To install the packages for use in a Go project:

```Go
$ go get github.com/mna/upstashdis
```

To install only the REST server command:

```Go
$ go install github.com/mna/upstashdis/cmd/upstash-redis-rest-server@latest
```

## Documentation

The [code documentation](https://pkg.go.dev/github.com/mna/upstashdis) is the canonical source for the Go packages documentation.

The `upstash-redis-rest-server` command documentation is available by running the command with the `--help` flag and is shown here as a convenience:

```
usage: upstash-redis-rest-server --addr <ADDR> --redis-addr <ADDR> [--api-token <TOKEN>]
       upstash-redis-rest-server --help

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
```

## License

The [BSD 3-Clause license](http://opensource.org/licenses/BSD-3-Clause).
