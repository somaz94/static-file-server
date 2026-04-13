package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.Folder != "/web" {
		t.Errorf("expected folder /web, got %s", cfg.Folder)
	}
	if !cfg.AllowIndex {
		t.Error("expected AllowIndex true")
	}
	if !cfg.ShowListing {
		t.Error("expected ShowListing true")
	}
	if cfg.CORS {
		t.Error("expected CORS false")
	}
	if cfg.Debug {
		t.Error("expected Debug false")
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yaml := `
cors: true
debug: true
host: "localhost"
port: 9090
folder: /data
allow-index: false
show-listing: false
url-prefix: "/files"
referrers:
  - "https://example.com"
  - "https://test.com"
access-key: "secret123"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.CORS {
		t.Error("expected CORS true")
	}
	if !cfg.Debug {
		t.Error("expected Debug true")
	}
	if cfg.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.Folder != "/data" {
		t.Errorf("expected folder /data, got %s", cfg.Folder)
	}
	if cfg.AllowIndex {
		t.Error("expected AllowIndex false")
	}
	if cfg.ShowListing {
		t.Error("expected ShowListing false")
	}
	if cfg.URLPrefix != "/files" {
		t.Errorf("expected URLPrefix /files, got %s", cfg.URLPrefix)
	}
	if len(cfg.Referrers) != 2 {
		t.Errorf("expected 2 referrers, got %d", len(cfg.Referrers))
	}
	if cfg.AccessKey != "secret123" {
		t.Errorf("expected access key secret123, got %s", cfg.AccessKey)
	}
}

func TestLoadEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yaml := `port: 9090`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PORT", "3000")
	t.Setenv("CORS", "yes")
	t.Setenv("DEBUG", "1")
	t.Setenv("FOLDER", "/var/www")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Port)
	}
	if !cfg.CORS {
		t.Error("expected CORS true from env")
	}
	if !cfg.Debug {
		t.Error("expected Debug true from env")
	}
	if cfg.Folder != "/var/www" {
		t.Errorf("expected folder /var/www, got %s", cfg.Folder)
	}
}

func TestLoadEnvBoolVariants(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		valid    bool
	}{
		{"1", true, true},
		{"true", true, true},
		{"TRUE", true, true},
		{"t", true, true},
		{"T", true, true},
		{"yes", true, true},
		{"YES", true, true},
		{"y", true, true},
		{"Y", true, true},
		{"0", false, true},
		{"false", false, true},
		{"FALSE", false, true},
		{"f", false, true},
		{"no", false, true},
		{"n", false, true},
		{"invalid", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := lookupEnvBool("TEST_BOOL")
			// Before setting env
			if ok {
				t.Error("expected not ok when env not set")
			}

			t.Setenv("TEST_BOOL", tt.input)
			got, ok = lookupEnvBool("TEST_BOOL")
			if ok != tt.valid {
				t.Errorf("input %q: expected valid=%v, got %v", tt.input, tt.valid, ok)
			}
			if ok && got != tt.expected {
				t.Errorf("input %q: expected %v, got %v", tt.input, tt.expected, got)
			}
		})
	}
}

func TestValidation_TLSMismatch(t *testing.T) {
	cfg := Default()
	cfg.TLSCert = "/path/to/cert.pem"
	// TLSKey is empty

	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for mismatched TLS cert/key")
	}
}

func TestValidation_InvalidTLSVersion(t *testing.T) {
	cfg := Default()
	cfg.TLSCert = "/cert.pem"
	cfg.TLSKey = "/key.pem"
	cfg.TLSMinVers = "TLS99"

	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for invalid TLS version")
	}
}

func TestValidation_URLPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		valid  bool
	}{
		{"/api", true},
		{"/my/prefix", true},
		{"", true},
		{"noslash", false},
		{"/trailing/", false},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			cfg := Default()
			cfg.URLPrefix = tt.prefix
			err := cfg.validate()
			if tt.valid && err != nil {
				t.Errorf("prefix %q: unexpected error: %v", tt.prefix, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("prefix %q: expected validation error", tt.prefix)
			}
		})
	}
}

func TestParseReferrers(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"https://example.com", []string{"https://example.com"}},
		{"https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}},
		{",https://a.com", []string{"", "https://a.com"}},
	}

	for _, tt := range tests {
		got := parseReferrers(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("input %q: expected %d items, got %d", tt.input, len(tt.expected), len(got))
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("input %q: item %d: expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

func TestHasTLS(t *testing.T) {
	cfg := Default()
	if cfg.HasTLS() {
		t.Error("expected HasTLS false with defaults")
	}

	cfg.TLSCert = "/cert.pem"
	cfg.TLSKey = "/key.pem"
	if !cfg.HasTLS() {
		t.Error("expected HasTLS true")
	}
}

func TestListenAddr(t *testing.T) {
	cfg := Default()
	if addr := cfg.ListenAddr(); addr != ":8080" {
		t.Errorf("expected :8080, got %s", addr)
	}

	cfg.Host = "localhost"
	cfg.Port = 3000
	if addr := cfg.ListenAddr(); addr != "localhost:3000" {
		t.Errorf("expected localhost:3000, got %s", addr)
	}
}

func TestSummary(t *testing.T) {
	cfg := Default()
	s := cfg.Summary()

	expected := []string{
		"Configuration:",
		"CORS:         false",
		"Debug:        false",
		`Host:         ""`,
		"Port:         8080",
		"Folder:       /web",
		"AllowIndex:   true",
		"ShowListing:  true",
		`URLPrefix:    ""`,
		"TLS:          false",
		"AccessKey:    false",
	}
	for _, e := range expected {
		if !strings.Contains(s, e) {
			t.Errorf("Summary missing %q", e)
		}
	}
}

func TestSummaryWithValues(t *testing.T) {
	cfg := Default()
	cfg.CORS = true
	cfg.Debug = true
	cfg.Host = "localhost"
	cfg.TLSCert = "/cert.pem"
	cfg.TLSKey = "/key.pem"
	cfg.AccessKey = "secret"
	cfg.Referrers = []string{"https://a.com", "https://b.com"}

	s := cfg.Summary()

	if !strings.Contains(s, "CORS:         true") {
		t.Error("Summary should show CORS true")
	}
	if !strings.Contains(s, "TLS:          true") {
		t.Error("Summary should show TLS true")
	}
	if !strings.Contains(s, "AccessKey:    true") {
		t.Error("Summary should show AccessKey true")
	}
	if !strings.Contains(s, "https://a.com") {
		t.Error("Summary should show referrers")
	}
}

func TestLoadEnvAllVariables(t *testing.T) {
	t.Setenv("CORS", "true")
	t.Setenv("DEBUG", "true")
	t.Setenv("HOST", "myhost")
	t.Setenv("PORT", "9090")
	t.Setenv("FOLDER", "/data")
	t.Setenv("ALLOW_INDEX", "false")
	t.Setenv("SHOW_LISTING", "false")
	t.Setenv("URL_PREFIX", "/api")
	t.Setenv("TLS_CERT", "/cert.pem")
	t.Setenv("TLS_KEY", "/key.pem")
	t.Setenv("TLS_MIN_VERS", "TLS12")
	t.Setenv("REFERRERS", "https://a.com,https://b.com")
	t.Setenv("ACCESS_KEY", "mykey")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.CORS {
		t.Error("expected CORS true")
	}
	if !cfg.Debug {
		t.Error("expected Debug true")
	}
	if cfg.Host != "myhost" {
		t.Errorf("expected host myhost, got %s", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.Folder != "/data" {
		t.Errorf("expected folder /data, got %s", cfg.Folder)
	}
	if cfg.AllowIndex {
		t.Error("expected AllowIndex false")
	}
	if cfg.ShowListing {
		t.Error("expected ShowListing false")
	}
	if cfg.URLPrefix != "/api" {
		t.Errorf("expected URLPrefix /api, got %s", cfg.URLPrefix)
	}
	if cfg.TLSCert != "/cert.pem" {
		t.Errorf("expected TLSCert /cert.pem, got %s", cfg.TLSCert)
	}
	if cfg.TLSKey != "/key.pem" {
		t.Errorf("expected TLSKey /key.pem, got %s", cfg.TLSKey)
	}
	if cfg.TLSMinVers != "TLS12" {
		t.Errorf("expected TLSMinVers TLS12, got %s", cfg.TLSMinVers)
	}
	if len(cfg.Referrers) != 2 || cfg.Referrers[0] != "https://a.com" {
		t.Errorf("expected referrers [https://a.com https://b.com], got %v", cfg.Referrers)
	}
	if cfg.AccessKey != "mykey" {
		t.Errorf("expected AccessKey mykey, got %s", cfg.AccessKey)
	}
}

func TestLoadEnvUint16Invalid(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Invalid PORT should be ignored, keeping default
	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080 for invalid PORT, got %d", cfg.Port)
	}
}

func TestLoadEnvUint16Overflow(t *testing.T) {
	t.Setenv("PORT", "99999")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Overflow should be ignored, keeping default
	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080 for overflow PORT, got %d", cfg.Port)
	}
}

func TestLoadYAMLNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent YAML file")
	}
}

func TestValidation_PortZero(t *testing.T) {
	cfg := Default()
	cfg.Port = 0
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for port 0")
	}
}
