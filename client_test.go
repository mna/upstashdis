package upstashdis

import (
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

	/*
		t.Run("pipeline Send Flush Receive", func(t *testing.T) {
			err := conn.Send("SET", "a", 1)
			require.NoError(t, err)
			err = conn.Send("APPEND", "a", "2")
			require.NoError(t, err)
			err = conn.Send("INCR", "a")
			require.NoError(t, err)
			err = conn.Flush()
			require.NoError(t, err)

			res1, err := redis.String(conn.Receive())
			require.NoError(t, err)
			require.Equal(t, "OK", res1)

			res2, err := redis.Int(conn.Receive())
			require.NoError(t, err)
			require.Equal(t, 2, res2)

			res3, err := redis.Int(conn.Receive())
			require.NoError(t, err)
			require.Equal(t, 13, res3)
		})
	*/
}
