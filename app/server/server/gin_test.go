package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"GOSpeak/internal/config"
)

func TestWSAllowedOriginsFallsBackToCORSOrigin(t *testing.T) {
	cfg := &config.Config{CORSOrigin: "*"}
	if got := wsAllowedOrigins(cfg); !reflect.DeepEqual(got, []string{"*"}) {
		t.Fatalf("expected CORS origin fallback, got %#v", got)
	}
}

func TestWSAllowedOriginsPrecedence(t *testing.T) {
	cfg := &config.Config{
		CORSOrigin:       "*",
		WSAllowedOrigins: "http://localhost:3000, https://voice.example.com",
	}
	if got := wsAllowedOrigins(cfg); !reflect.DeepEqual(got, []string{
		"http://localhost:3000",
		"https://voice.example.com",
	}) {
		t.Fatalf("expected explicit WS origins, got %#v", got)
	}
}

func TestServeHTTPRequiresBothTLSFiles(t *testing.T) {
	srv := &http.Server{}
	if err := serveHTTP(srv, "/tmp/gospeak-cert.pem", ""); err == nil || !strings.Contains(err.Error(), "TLS_CERT and TLS_KEY") {
		t.Fatalf("expected paired TLS config error, got %v", err)
	}
	if err := serveHTTP(srv, "", "/tmp/gospeak-key.pem"); err == nil || !strings.Contains(err.Error(), "TLS_CERT and TLS_KEY") {
		t.Fatalf("expected paired TLS config error, got %v", err)
	}
}

func TestTLSServerSupportsHTTP2WithHTTP11Fallback(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ServeTLS(ln, certFile, keyFile)
	}()
	defer func() {
		_ = srv.Close()
		if err := <-serveErr; err != nil && err != http.ErrServerClosed {
			t.Errorf("ServeTLS: %v", err)
		}
	}()

	url := "https://" + ln.Addr().String() + "/ping"

	h2Resp, err := (&http.Client{Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true,
	}}).Get(url)
	if err != nil {
		t.Fatalf("h2 request: %v", err)
	}
	defer h2Resp.Body.Close()
	if h2Resp.Proto != "HTTP/2.0" {
		t.Fatalf("expected HTTP/2.0, got %s", h2Resp.Proto)
	}

	h1Resp, err := (&http.Client{Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true,
		TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
	}}).Get(url)
	if err != nil {
		t.Fatalf("http/1.1 fallback request: %v", err)
	}
	defer h1Resp.Body.Close()
	if h1Resp.Proto != "HTTP/1.1" {
		t.Fatalf("expected HTTP/1.1 fallback, got %s", h1Resp.Proto)
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}
