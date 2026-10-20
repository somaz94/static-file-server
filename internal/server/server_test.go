package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/somaz94/static-file-server/internal/config"
)

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

// startRun runs Run in the background; the returned stop cancels it and waits for its
// error, and also runs at test cleanup so no server outlives its test.
func startRun(t *testing.T, cfg *config.Config) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg) }()
	var once sync.Once
	var err error
	stop = func() error {
		once.Do(func() {
			cancel()
			select {
			case err = <-errCh:
			case <-time.After(5 * time.Second):
				err = errors.New("server did not shut down within 5 seconds")
			}
		})
		return err
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

func TestRunHTTP(t *testing.T) {
	dir := setupTestFolder(t)
	port := freePort(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir

	startRun(t, cfg)

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

	startRun(t, cfg)

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

	startRun(t, cfg)

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

	stop := startRun(t, cfg)

	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForServer(t, http.DefaultClient, addr+"/healthz")

	if err := stop(); err != nil {
		t.Errorf("expected nil error from graceful shutdown, got %v", err)
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

	stop := startRun(t, cfg)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	addr := fmt.Sprintf("https://127.0.0.1:%d", port)
	waitForServer(t, client, addr+"/healthz")

	if err := stop(); err != nil {
		t.Errorf("expected nil error from graceful shutdown, got %v", err)
	}
}

func TestRunWithS3Cache(t *testing.T) {
	dir := setupTestFolder(t)
	port, s3Port := freePort(t), freePort(t)

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Folder = dir
	cfg.S3Enabled = true
	cfg.S3Port = s3Port
	cfg.S3Folder = t.TempDir()
	cfg.S3AccessKey = "ak"
	cfg.S3SecretKey = "sk"
	cfg.S3TTL = time.Hour

	stop := startRun(t, cfg)

	s3Addr := fmt.Sprintf("http://127.0.0.1:%d", s3Port)
	waitForServer(t, http.DefaultClient, s3Addr+"/healthz")
	waitForServer(t, http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d/healthz", port))

	resp, err := http.Get(s3Addr + "/runner-cache/key")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("unsigned S3 request: expected 403, got %d", resp.StatusCode)
	}

	if err := stop(); err != nil {
		t.Errorf("expected nil error from graceful shutdown, got %v", err)
	}
}

func TestRunS3PortInUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = freePort(t)
	cfg.Folder = setupTestFolder(t)
	cfg.S3Enabled = true
	cfg.S3Port = uint16(l.Addr().(*net.TCPAddr).Port)
	cfg.S3Folder = t.TempDir()
	cfg.S3AccessKey = "ak"
	cfg.S3SecretKey = "sk"

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(t.Context(), cfg)
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "S3 cache "+cfg.S3Folder+": ") {
			t.Errorf("expected a labelled error when the S3 port is taken, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not fail on a taken S3 port")
	}
}

func TestRunCacheOnly(t *testing.T) {
	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = freePort(t)
	s3Port := freePort(t)
	cfg.S3Port = s3Port
	cfg.S3Folder = t.TempDir()
	cfg.S3AccessKey = "ak"
	cfg.S3SecretKey = "sk"
	if err := cfg.EnableCacheOnly(); err != nil {
		t.Fatal(err)
	}

	stop := startRun(t, cfg)

	waitForServer(t, http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d/healthz", s3Port))
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port), time.Second); err == nil {
		conn.Close()
		t.Error("cache-only mode must not start the file server")
	}

	if err := stop(); err != nil {
		t.Errorf("expected nil error from graceful shutdown, got %v", err)
	}
}

func TestRunNothingToServe(t *testing.T) {
	cfg := config.Default()
	cfg.CacheOnly = true
	if err := Run(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "nothing to serve") {
		t.Fatalf("expected a nothing-to-serve error, got %v", err)
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		format     string
		debug      bool
		wantDebug  bool
		wantPrefix string
	}{
		{"text", false, false, "time="},
		{"text", true, true, "time="},
		{"json", false, false, "{"},
		{"json", true, true, "{"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/debug=%t", tt.format, tt.debug), func(t *testing.T) {
			var buf bytes.Buffer
			cfg := config.Default()
			cfg.LogFormat, cfg.Debug = tt.format, tt.debug
			logger := NewLogger(&buf, cfg)
			logger.Debug("debug-line")
			logger.Info("info-line")
			out := buf.String()
			if strings.Contains(out, "debug-line") != tt.wantDebug || !strings.Contains(out, "info-line") || !strings.HasPrefix(out, tt.wantPrefix) {
				t.Fatalf("unexpected output: %s", out)
			}
		})
	}
}

func TestRunLogsLifecycle(t *testing.T) {
	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = freePort(t)
	cfg.Folder = setupTestFolder(t)
	cfg.Debug = true

	var buf syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx, cfg, NewLogger(&buf, cfg)) }()
	waitForServer(t, http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d/test.txt", cfg.Port))
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{"msg=configuration", "msg=serving", "msg=request", `msg="shutting down"`, `msg="server stopped"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log missing %q: %s", want, buf.String())
		}
	}
}

// syncBuffer is a bytes.Buffer safe for the concurrent writes of a running server.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
