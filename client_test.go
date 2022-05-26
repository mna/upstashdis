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
		req := cli.NewRequest()
		err := req.Send("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty command")
	})

	t.Run("override token with invalid one", func(t *testing.T) {
		req := cli.NewRequestWithToken("nosuchtoken")
		err := req.ExecOne(nil, "ECHO", "a")
		require.Error(t, err)
		require.Contains(t, err.Error(), "Unauthorized")
	})

	t.Run("clone client", func(t *testing.T) {
		cli2 := cli.CloneWithToken("nosuchtoken")
		req := cli2.NewRequest()
		err := req.ExecOne(nil, "ECHO", "a")
		require.Error(t, err)
		require.Contains(t, err.Error(), "Unauthorized")

		req = cli2.NewRequestWithToken(tok)
		err = req.ExecOne(nil, "ECHO", "a")
		require.NoError(t, err)
	})

	t.Run("empty ExecOne", func(t *testing.T) {
		var got string
		req := cli.NewRequest()
		err := req.ExecOne(&got, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty command")
	})

	t.Run("simple ExecOne", func(t *testing.T) {
		var got string
		req := cli.NewRequest()
		err := req.ExecOne(&got, "ECHO", "a")
		require.NoError(t, err)
		require.Equal(t, "a", got)
	})

	t.Run("fail ExecOne", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.ExecOne(nil, "NOTACMD", "a")
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 0, rerr.PipelineIndex)
	})

	t.Run("ExecOne with queued", func(t *testing.T) {
		var got string
		req := cli.NewRequest()
		err := req.Send("ECHO", "a")
		require.NoError(t, err)
		err = req.ExecOne(&got, "ECHO", "b")
		require.NoError(t, err)
		require.Equal(t, "b", got)
	})

	t.Run("ExecOne with failed queued", func(t *testing.T) {
		var got string
		req := cli.NewRequest()
		err := req.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = req.ExecOne(&got, "ECHO", "b")
		require.NoError(t, err)
		require.Equal(t, "b", got)
	})

	t.Run("ExecOne fail with failed queued", func(t *testing.T) {
		var got string
		req := cli.NewRequest()
		err := req.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = req.ExecOne(&got, "STILLNOTACMD", "b")
		require.Error(t, err)
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 0, rerr.PipelineIndex)
	})

	t.Run("Exec single", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.Send("ECHO", "a")
		require.NoError(t, err)

		var echo string
		err = req.Exec(&echo) // ignore the return value of SET
		require.NoError(t, err)
		require.Equal(t, "a", echo)
	})

	t.Run("Exec pipeline", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.Send("SET", "k", 1)
		require.NoError(t, err)
		err = req.Send("INCR", "k")
		require.NoError(t, err)
		err = req.Send("GET", "k")
		require.NoError(t, err)

		var incr int
		var get string
		err = req.Exec(nil, &incr, &get) // ignore the return value of SET
		require.NoError(t, err)
		require.Equal(t, 2, incr)
		require.Equal(t, "2", get)
	})

	t.Run("Exec single failed", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.Send("NOTACMD", "a")
		require.NoError(t, err)

		err = req.Exec()
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		var rerr *Error
		require.True(t, errors.As(err, &rerr))
		require.Equal(t, "ERR", rerr.Kind)
		require.Equal(t, 0, rerr.PipelineIndex)
	})

	t.Run("Exec pipeline failed", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.Send("SET", "k", 1)
		require.NoError(t, err)
		err = req.Send("INCR", "k")
		require.NoError(t, err)
		err = req.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = req.Send("GET", "k")
		require.NoError(t, err)

		var incr int
		var raw json.RawMessage
		var get string
		err = req.Exec(nil, &incr, &raw, &get)
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
		req := cli.NewRequest()
		err := req.Send("SET", "k", 1)
		require.NoError(t, err)
		err = req.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = req.Send("NOTACMD", "b")
		require.NoError(t, err)
		err = req.Send("GET", "k")
		require.NoError(t, err)

		var raw1, raw2 json.RawMessage
		var get string
		err = req.Exec(nil, &raw1, &raw2, &get)
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
		req := cli.NewRequest()
		err := req.Send("SET", "k", 1)
		require.NoError(t, err)

		var v1, v2 string
		err = req.Exec(&v1, &v2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too many destination values")
		var rerr *Error
		require.False(t, errors.As(err, &rerr))
	})

	t.Run("Exec pipeline less dst", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.Send("SET", "k", 1)
		require.NoError(t, err)
		err = req.Send("INCR", "k")
		require.NoError(t, err)
		err = req.Send("GET", "k")
		require.NoError(t, err)

		var v1 string
		err = req.Exec(&v1)
		require.NoError(t, err)
		require.Equal(t, "OK", v1)
	})

	t.Run("ExecRaw pipeline with failures", func(t *testing.T) {
		req := cli.NewRequest()
		err := req.Send("SET", "k", 1)
		require.NoError(t, err)
		err = req.Send("INCR", "k")
		require.NoError(t, err)
		err = req.Send("NOTACMD", "a")
		require.NoError(t, err)
		err = req.Send("GET", "k")
		require.NoError(t, err)
		err = req.Send("NOTACMD", "b")
		require.NoError(t, err)

		res, err := req.ExecRaw()
		require.NoError(t, err)
		require.Len(t, res, 5)
		require.Empty(t, res[0].Error)
		require.Empty(t, res[1].Error)
		require.NotEmpty(t, res[2].Error)
		require.Empty(t, res[3].Error)
		require.NotEmpty(t, res[4].Error)
	})
}
