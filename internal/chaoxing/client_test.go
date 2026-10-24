package chaoxing

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := NewClient(filepath.Join(t.TempDir(), "session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(server.URL)
	c.HTTP.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		copy := req.Clone(req.Context())
		u := *req.URL
		u.Scheme = target.Scheme
		u.Host = target.Host
		copy.URL = &u
		r, e := http.DefaultTransport.RoundTrip(copy)
		if r != nil {
			r.Request = req
		}
		return r, e
	})
	return c
}
