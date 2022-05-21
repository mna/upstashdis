[![Go Reference](https://pkg.go.dev/badge/github.com/mna/upstashdis.svg)](https://pkg.go.dev/github.com/mna/upstashdis)
[![Build Status](https://github.com/mna/upstashdis/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/mna/upstashdis/actions)

# upstashdis

Package `upstashdis` provides a Go client for the [Upstash Redis REST API](https://docs.upstash.com/redis/features/restapi) interface. Note that this package is *not* affiliated with Upstash. It also provides a `restserver` and a `upstash-redis-rest-server` Go package and executable command (respectively) to run a local web server that serves an Upstash-compatible REST API in front of an actual Redis database instance, for testing purposes.

## Installation

Note that Go1.18+ is required. To install the packages for use in a Go project:

```Go
$ go get github.com/mna/upstashdis
```

To install only the command:

```Go
$ go install github.com/mna/upstashdis/cmd/upstash-redis-rest-server@latest
```

## Documentation

The [code documentation](https://pkg.go.dev/github.com/mna/upstashdis) is the canonical source for the Go packages documentation.

The `upstash-redis-rest-server` command documentation is available by running the command with the `--help` flag and is show here for reference:

```
TODO: paste help text.
```

## License

The [BSD 3-Clause license](http://opensource.org/licenses/BSD-3-Clause).
