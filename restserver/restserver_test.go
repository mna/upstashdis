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
	Result  interface{} `json:"result"`
	Results []result    `json:"results"`
	Error   string      `json:"error"`
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
		req, err := http.NewRequest("POST", u.String(), rbody)
		require.NoError(t, err)

		if token != "" {
			req.Header.Add("Authorization", "Bearer "+token)
		}

		res, err := cli.Do(req)
		require.NoError(t, err)

		resBody, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		require.Equal(t, code, res.StatusCode, string(resBody))

		var restResult result
		if path == "/pipeline" {
			require.NoError(t, json.Unmarshal(resBody, &restResult.Results))
		} else {
			require.NoError(t, json.Unmarshal(resBody, &restResult))
		}
		return restResult
	}

	t.Run("missing auth", func(t *testing.T) {
		res := makeRequest(t, http.StatusUnauthorized, "", "/echo/a", nil)
		require.Contains(t, res.Error, "Unauthorized")
	})

	t.Run("bad token", func(t *testing.T) {
		res := makeRequest(t, http.StatusUnauthorized, badToken, "/echo/a", nil)
		require.Contains(t, res.Error, "Unauthorized")
	})

	t.Run("good token", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/echo/a", nil)
		require.Empty(t, res.Error)
		require.Equal(t, res.Result, "a")
	})

	t.Run("hgetall", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/pipeline", [][]string{{"HSET", "h1", "a", "1"}, {"HSET", "h1", "b", "2"}, {"HGETALL", "h1"}})
		require.Empty(t, res.Error)
		require.Len(t, res.Results, 3)
		require.Equal(t, res.Results[2], result{Result: []interface{}{"a", "1", "b", "2"}})
	})
}
