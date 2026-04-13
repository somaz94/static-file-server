package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/somaz94/static-file-server/internal/config"
)

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		{"TLS11", tls.VersionTLS11},
		{"TLS12", tls.VersionTLS12},
		{"TLS13", tls.VersionTLS13},
		{"tls12", tls.VersionTLS12},     // case insensitive
		{"  TLS13  ", tls.VersionTLS13}, // whitespace trimmed
		{"", tls.VersionTLS12},          // empty defaults to TLS12
		{"invalid", tls.VersionTLS12},   // invalid defaults to TLS12
		{"TLS99", tls.VersionTLS12},     // unknown defaults to TLS12
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseTLSVersion(tt.input)
			if got != tt.expected {
				t.Errorf("parseTLSVersion(%q): expected 0x%04x, got 0x%04x",
					tt.input, tt.expected, got)
			}
		})
	}
}

// freePort finds an available TCP port.
func freePort(t *testing.T) uint16 {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return uint16(port)
}

// setupTestFolder creates a temp directory with a test file.
func setupTestFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello server"), 0644)
	return dir
}

// waitForServer polls the given URL until it responds or times out.
func waitForServer(t *testing.T, client *http.Client, url string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready", url)
}

func TestRunHTTP(t *testing.T) {
	dir := setupTestFolder(t)
	port := freePort(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(cfg)
	}()

	// Wait for server to start
	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = http.Get(addr + "/test.txt")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server did not start: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "hello server" {
		t.Errorf("expected 'hello server', got %q", string(body))
	}
}

func TestRunHTTPWithDebug(t *testing.T) {
	dir := setupTestFolder(t)
	port := freePort(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir
	cfg.Debug = true

	go func() {
		Run(cfg)
	}()

	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = http.Get(addr + "/test.txt")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server did not start: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// generateSelfSignedCert creates a temporary self-signed certificate and key.
func generateSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	certFile, _ := os.Create(certPath)
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certFile.Close()

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}

	keyPath = filepath.Join(dir, "key.pem")
	keyFile, _ := os.Create(keyPath)
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyFile.Close()

	return certPath, keyPath
}

func TestRunTLS(t *testing.T) {
	dir := setupTestFolder(t)
	port := freePort(t)
	certPath, keyPath := generateSelfSignedCert(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir
	cfg.TLSCert = certPath
	cfg.TLSKey = keyPath
	cfg.TLSMinVers = "TLS12"

	go func() {
		Run(cfg)
	}()

	// Use a client that skips TLS verification for self-signed cert
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	addr := fmt.Sprintf("https://127.0.0.1:%d", port)
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = client.Get(addr + "/test.txt")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("TLS server did not start: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "hello server" {
		t.Errorf("expected 'hello server', got %q", string(body))
	}
}

func TestGracefulShutdown(t *testing.T) {
	dir := setupTestFolder(t)
	port := freePort(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(cfg)
	}()

	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForServer(t, http.DefaultClient, addr+"/healthz")

	// Send SIGINT to trigger graceful shutdown
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error from graceful shutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5 seconds")
	}
}

func TestGracefulShutdownTLS(t *testing.T) {
	dir := setupTestFolder(t)
	port := freePort(t)
	certPath, keyPath := generateSelfSignedCert(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir
	cfg.TLSCert = certPath
	cfg.TLSKey = keyPath
	cfg.TLSMinVers = "TLS12"

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(cfg)
	}()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	addr := fmt.Sprintf("https://127.0.0.1:%d", port)
	waitForServer(t, client, addr+"/healthz")

	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error from graceful shutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TLS server did not shut down within 5 seconds")
	}
}
