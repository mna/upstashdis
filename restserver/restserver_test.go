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

	miniredis "github.com/alicebob/miniredis/v2"
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
	makeRequest := func(t *testing.T, code int, token, path string, body interface{}, rawQuery string) result {
		u, err := url.Parse(httpsrv.URL)
		require.NoError(t, err)
		u.Path = path

		if rawQuery != "" {
			u.RawQuery = rawQuery
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
		if path == "/pipeline" && res.StatusCode == http.StatusOK {
			require.NoError(t, json.Unmarshal(resBody, &restResult.Results), string(resBody))
		} else {
			require.NoError(t, json.Unmarshal(resBody, &restResult))
		}
		return restResult
	}

	t.Run("missing auth", func(t *testing.T) {
		res := makeRequest(t, http.StatusUnauthorized, "", "/echo/a", nil, "")
		require.Contains(t, res.Error, "Unauthorized")
	})

	t.Run("bad token", func(t *testing.T) {
		res := makeRequest(t, http.StatusUnauthorized, badToken, "/echo/a", nil, "")
		require.Contains(t, res.Error, "Unauthorized")
	})

	t.Run("good token", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/echo/a", nil, "")
		require.Empty(t, res.Error)
		require.Equal(t, res.Result, "a")
	})

	t.Run("no command", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/", nil, "")
		require.Contains(t, res.Error, "failed to parse command")
	})

	t.Run("invalid command", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/", true, "")
		require.Contains(t, res.Error, "failed to parse command")
	})

	t.Run("empty command", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/", []interface{}{}, "")
		require.Contains(t, res.Error, "empty command")
	})

	t.Run("unknown body command", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/", []interface{}{"NOPE"}, "")
		require.Contains(t, res.Error, "unknown command")
	})

	t.Run("unknown path command", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/nope", nil, "")
		require.Contains(t, res.Error, "unknown command")
	})

	t.Run("path and query params", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/set/a", nil, "test&EX=10")
		require.Empty(t, res.Error)
		require.Equal(t, res.Result, "OK")

		v, err := redsrv.Get("a")
		require.NoError(t, err)
		require.Equal(t, "test", v)
		dur := redsrv.TTL("a")
		require.Greater(t, dur, 5*time.Second)
	})

	t.Run("path and body", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/set/a", "test", "")
		require.Empty(t, res.Error)
		require.Equal(t, res.Result, "OK")

		v, err := redsrv.Get("a")
		require.NoError(t, err)
		require.Equal(t, `"test"`, v)
	})

	t.Run("path and body and query params", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/set/a", "bodyquery", "EX=20")
		require.Empty(t, res.Error)
		require.Equal(t, res.Result, "OK")

		v, err := redsrv.Get("a")
		require.NoError(t, err)
		require.Equal(t, `"bodyquery"`, v)
		dur := redsrv.TTL("a")
		require.Greater(t, dur, 15*time.Second)
	})

	t.Run("valid body command", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/", []string{"echo", "a"}, "")
		require.Empty(t, res.Error)
		require.Equal(t, res.Result, "a")
	})

	t.Run("no pipeline command", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/pipeline", nil, "")
		require.Contains(t, res.Error, "failed to parse pipeline request")
	})

	t.Run("empty pipeline", func(t *testing.T) {
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/pipeline", [][]interface{}{}, "")
		require.Contains(t, res.Error, "empty pipeline request")
	})

	t.Run("empty command in pipeline", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/pipeline", [][]interface{}{{}}, "")
		require.Len(t, res.Results, 1)
		require.Contains(t, res.Results[0].Error, "empty pipeline command")
	})

	t.Run("valid pipeline", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/pipeline", [][]interface{}{{"SET", "a", "test1"}, {"GET", "a"}}, "")
		require.Len(t, res.Results, 2)
		require.Equal(t, "OK", res.Results[0].Result)
		require.Equal(t, "test1", res.Results[1].Result)
	})

	t.Run("pipeline with failure", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/pipeline", [][]interface{}{{"SET", "a", "test2"}, {"HGETALL", "a"}, {"GET", "a"}}, "")
		require.Len(t, res.Results, 3)
		require.Equal(t, "OK", res.Results[0].Result)
		require.Contains(t, res.Results[1].Error, "WRONGTYPE")
		require.Equal(t, "test2", res.Results[2].Result)
	})

	t.Run("hgetall", func(t *testing.T) {
		res := makeRequest(t, http.StatusOK, goodToken, "/pipeline", [][]string{{"HSET", "h1", "a", "1"}, {"HSET", "h1", "b", "2"}, {"HGETALL", "h1"}}, "")
		require.Empty(t, res.Error)
		require.Len(t, res.Results, 3)
		require.Equal(t, res.Results[2], result{Result: []interface{}{"a", "1", "b", "2"}})
	})

	t.Run("acl resttoken invalid user", func(t *testing.T) {
		redsrv.RequireUserAuth("user", "pwd")
		defer redsrv.RequireUserAuth("user", "")
		res := makeRequest(t, http.StatusBadRequest, goodToken, "/acl/resttoken/user/WRONGPWD", nil, "")
		require.Contains(t, res.Error, "WRONGPASS")
	})

	// TODO: this one would require a real redis instance
	//t.Run("acl resttoken valid user", func(t *testing.T) {
	//	redsrv.RequireUserAuth("user", "pwd")
	//	defer redsrv.RequireUserAuth("user", "")
	//	res := makeRequest(t, http.StatusBadRequest, goodToken, "/acl/resttoken/user/pwd", nil, "")
	//	require.Empty(t, res.Error)
	//	require.NotEmpty(t, res.Result)
	//})
}
