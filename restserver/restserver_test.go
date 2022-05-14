package restserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/require"
)

type result struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error"`
}

func TestServer(t *testing.T) {
	redsrv := miniredis.RunT(t)
	pool := redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redsrv.Addr())
		},
	}
	defer pool.Close()

	const goodToken, badToken = "_token_", "_badtoken_"
	server := &Server{
		APIToken: goodToken,
		GetConnFunc: func(ctx context.Context) Conn {
			return pool.Get()
		},
	}

	httpsrv := httptest.NewServer(server)
	defer httpsrv.Close()

	cli := &http.Client{Timeout: 5 * time.Second}
	makeRequest := func(t *testing.T, code int, token, path string, body interface{}, queries ...string) result {
		u, err := url.Parse(httpsrv.URL)
		require.NoError(t, err)
		u.Path = path

		if len(queries) > 0 {
			qvals := make(url.Values)
			for i := 0; i < len(queries)/2; i++ {
				k := queries[i]
				var v string
				if i+1 < len(queries) {
					v = queries[i+1]
				}
				qvals.Add(k, v)
			}
			u.RawQuery = qvals.Encode()
		}

		var rbody io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			require.NoError(t, err)
			rbody = bytes.NewReader(b)
		}
		req := httptest.NewRequest("POST", u.String(), rbody)

		if token != "" {
			req.Header.Add("Authorization", "Bearer "+token)
		}

		res, err := cli.Do(req)
		require.NoError(t, err)
		require.Equal(t, code, res.StatusCode)

		var restResult result
		require.NoError(t, json.NewDecoder(res.Body).Decode(&restResult))
		return restResult
	}

	t.Run("missing auth", func(t *testing.T) {
		res := makeRequest(t, http.StatusUnauthorized, "", "/echo/a", nil)
		require.Contains(t, res.Error, "unauthorized")
	})
}
