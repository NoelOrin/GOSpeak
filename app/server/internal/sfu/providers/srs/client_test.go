package srs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ListStreams_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":0,"streams":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.ListStreams(); err == nil {
		t.Fatal("ListStreams should error on non-2xx response")
	}
}

func TestClient_FetchClients_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":0,"clients":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.fetchClients(); err == nil {
		t.Fatal("fetchClients should error on non-2xx response")
	}
}

func TestClient_DeleteRoom_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.DeleteRoom("room-x"); err == nil {
		t.Fatal("DeleteRoom should error on non-2xx response")
	}
}
