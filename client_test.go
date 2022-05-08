package upstashdis

import (
	"os"
	"testing"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/require"
)

func TestUpstash(t *testing.T) {
	url := os.Getenv("UPSTASHDIS_TEST_REST_URL")
	tok := os.Getenv("UPSTASHDIS_TEST_REST_TOKEN")
	if url == "" || tok == "" {
		t.Skip("environment variables not set, set UPSTASHDIS_TEST_REST_{URL,TOKEN} to a valid upstash Redis instance")
	}

	cli := &Client{BaseURL: url, APIToken: tok}
	conn := cli.NewConn()
	defer conn.Close()

	t.Run("empty Do", func(t *testing.T) {
		res, err := conn.Do("")
		require.NoError(t, err)
		require.Nil(t, res)
	})

	t.Run("simple Do", func(t *testing.T) {
		res, err := redis.String(conn.Do("ECHO", "a"))
		require.NoError(t, err)
		require.Equal(t, "a", res)
	})

	t.Run("fail Do", func(t *testing.T) {
		res, err := conn.Do("NOTACMD", "a")
		require.Error(t, err)
		require.Contains(t, err.Error(), "ERR Command is not available")
		require.Nil(t, res)
	})

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
}
