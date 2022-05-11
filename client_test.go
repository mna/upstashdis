package upstashdis

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstash(t *testing.T) {
	url := os.Getenv("UPSTASHDIS_TEST_REST_URL")
	tok := os.Getenv("UPSTASHDIS_TEST_REST_TOKEN")
	if url == "" || tok == "" {
		t.Skip("environment variables not set, set UPSTASHDIS_TEST_REST_{URL,TOKEN} to a valid upstash Redis instance")
	}

	cli := &Client{BaseURL: url, APIToken: tok}

	t.Run("empty Send", func(t *testing.T) {
		err := cli.Send("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty command")
	})

	t.Run("empty ExecOne", func(t *testing.T) {
		var got string
		err := cli.ExecOne(&got, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty command")
	})

	t.Run("simple ExecOne", func(t *testing.T) {
		var got string
		err := cli.ExecOne(&got, "ECHO", "a")
		require.NoError(t, err)
		require.Equal(t, "a", got)
	})

	t.Run("fail ExecOne", func(t *testing.T) {
		err := cli.ExecOne(nil, "NOTACMD", "a")
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 0, rerr.PipelineIndex)
	})

	t.Run("ExecOne with queued", func(t *testing.T) {
		var got string
		err := cli.Send("ECHO", "a")
		require.NoError(t, err)
		err = cli.ExecOne(&got, "ECHO", "b")
		require.NoError(t, err)
		require.Equal(t, "b", got)
	})

	t.Run("ExecOne with failed queued", func(t *testing.T) {
		var got string
		err := cli.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = cli.ExecOne(&got, "ECHO", "b")
		require.NoError(t, err)
		require.Equal(t, "b", got)
	})

	t.Run("ExecOne fail with failed queued", func(t *testing.T) {
		var got string
		err := cli.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = cli.ExecOne(&got, "STILLNOTACMD", "b")
		require.Error(t, err)
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 0, rerr.PipelineIndex)
	})

	t.Run("Exec single", func(t *testing.T) {
		err := cli.Send("ECHO", "a")
		require.NoError(t, err)

		var echo string
		err = cli.Exec(&echo) // ignore the return value of SET
		require.NoError(t, err)
		require.Equal(t, "a", echo)
	})

	t.Run("Exec pipeline", func(t *testing.T) {
		err := cli.Send("SET", "k", 1)
		require.NoError(t, err)
		err = cli.Send("INCR", "k")
		require.NoError(t, err)
		err = cli.Send("GET", "k")
		require.NoError(t, err)

		var incr int
		var get string
		err = cli.Exec(nil, &incr, &get) // ignore the return value of SET
		require.NoError(t, err)
		require.Equal(t, 2, incr)
		require.Equal(t, "2", get)
	})

	t.Run("Exec single failed", func(t *testing.T) {
		err := cli.Send("NOTACMD", "a")
		require.NoError(t, err)

		err = cli.Exec()
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 0, rerr.PipelineIndex)
	})

	t.Run("Exec pipeline failed", func(t *testing.T) {
		err := cli.Send("SET", "k", 1)
		require.NoError(t, err)
		err = cli.Send("INCR", "k")
		require.NoError(t, err)
		err = cli.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = cli.Send("GET", "k")
		require.NoError(t, err)

		var incr int
		var raw json.RawMessage
		var get string
		err = cli.Exec(nil, &incr, &raw, &get)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 2, rerr.PipelineIndex)

		// results were still returned
		require.Equal(t, 2, incr)
		require.Nil(t, raw)
		require.Equal(t, "2", get)
	})

	t.Run("Exec pipeline multi-failed", func(t *testing.T) {
		err := cli.Send("SET", "k", 1)
		require.NoError(t, err)
		err = cli.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = cli.Send("NOTACMD", "b")
		require.NoError(t, err)
		err = cli.Send("GET", "k")
		require.NoError(t, err)

		var raw1, raw2 json.RawMessage
		var get string
		err = cli.Exec(nil, &raw1, &raw2, &get)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 1, rerr.PipelineIndex)

		// results were still returned
		require.Nil(t, raw1)
		require.Nil(t, raw2)
		require.Equal(t, "1", get)
	})

	t.Run("Exec pipeline too many dst", func(t *testing.T) {
		err := cli.Send("SET", "k", 1)
		require.NoError(t, err)

		var v1, v2 string
		err = cli.Exec(&v1, &v2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too many destination values")
		var rerr *Error
		require.False(t, errors.As(err, &rerr))
	})

	t.Run("Exec pipeline less dst", func(t *testing.T) {
		err := cli.Send("SET", "k", 1)
		require.NoError(t, err)
		err = cli.Send("INCR", "k")
		require.NoError(t, err)
		err = cli.Send("GET", "k")
		require.NoError(t, err)

		var v1 string
		err = cli.Exec(&v1)
		require.NoError(t, err)
		require.Equal(t, "OK", v1)
	})

	t.Run("ExecRaw pipeline with failures", func(t *testing.T) {
		err := cli.Send("SET", "k", 1)
		require.NoError(t, err)
		err = cli.Send("INCR", "k")
		require.NoError(t, err)
		err = cli.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = cli.Send("GET", "k")
		require.NoError(t, err)
		err = cli.Send("NOTACMD", "b")
		require.NoError(t, err)

		res, err := cli.ExecRaw()
		require.NoError(t, err)
		require.Len(t, res, 5)
		require.Empty(t, res[0].Error)
		require.Empty(t, res[1].Error)
		require.NotEmpty(t, res[2].Error)
		require.Empty(t, res[3].Error)
		require.NotEmpty(t, res[4].Error)
	})
}
