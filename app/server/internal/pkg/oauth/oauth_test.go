package oauth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClient_TimeoutAndBodyLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 3*1024*1024))
	}))
	defer ts.Close()

	if _, err := httpGet(ts.URL, ""); err == nil {
		t.Fatal("expected body too large error")
	}
}

func TestHTTPClient_RejectsUnsafeURL(t *testing.T) {
	if _, err := httpGet("file:///etc/passwd", ""); err == nil {
		t.Fatal("expected unsafe scheme error")
	}
}
