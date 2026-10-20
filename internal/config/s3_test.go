package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validS3Config(t *testing.T) *Config {
	t.Helper()
	root := t.TempDir()
	cfg := Default()
	cfg.Folder = filepath.Join(root, "web")
	cfg.S3Enabled = true
	cfg.S3Folder = filepath.Join(root, "cache")
	cfg.S3AccessKey = "ak"
	cfg.S3SecretKey = "sk"
	return cfg
}

func TestDefaultS3(t *testing.T) {
	cfg := Default()
	if cfg.S3Enabled || cfg.S3Port != 9000 {
		t.Fatalf("unexpected S3 defaults: enabled=%v port=%d", cfg.S3Enabled, cfg.S3Port)
	}
}

func TestLoadEnvS3(t *testing.T) {
	t.Setenv("S3_ENABLED", "true")
	t.Setenv("S3_PORT", "9100")
	t.Setenv("FOLDER", "/web")
	t.Setenv("S3_FOLDER", "/cache")
	t.Setenv("S3_ACCESS_KEY", "ak")
	t.Setenv("S3_SECRET_KEY", "sk")
	t.Setenv("S3_BUCKETS", "runner-cache, other")
	t.Setenv("S3_TTL", "336h")
	t.Setenv("S3_MAX_OBJECT_SIZE", "1073741824")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.S3Enabled || cfg.S3Port != 9100 || cfg.S3Folder != "/cache" || cfg.S3AccessKey != "ak" ||
		cfg.S3SecretKey != "sk" || strings.Join(cfg.S3Buckets, "|") != "runner-cache|other" ||
		cfg.S3TTL != 336*time.Hour || cfg.S3MaxObjectSize != 1<<30 {
		t.Fatalf("unexpected S3 config: %+v", cfg)
	}
	if cfg.S3ListenAddr() != ":9100" {
		t.Fatalf("S3ListenAddr = %q", cfg.S3ListenAddr())
	}
}

func TestLoadEnvS3InvalidNumbersRejected(t *testing.T) {
	tests := []struct{ key, value string }{
		{"S3_TTL", "7d"},
		{"S3_TTL", "two weeks"},
		{"S3_MAX_OBJECT_SIZE", "1G"},
		{"S3_MAX_TOTAL_SIZE", "50Gi"},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			_, err := Load("")
			if err == nil || !strings.Contains(err.Error(), tt.key+"=") {
				t.Fatalf("expected an error naming %s, got %v", tt.key, err)
			}
		})
	}
}

func TestLoadYAMLS3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "folder: /web\ns3-enabled: true\ns3-folder: /cache\ns3-access-key: ak\ns3-secret-key: sk\n" +
		"s3-buckets: [runner-cache]\ns3-ttl: 72h\ns3-max-object-size: 100\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.S3Enabled || cfg.S3TTL != 72*time.Hour || cfg.S3MaxObjectSize != 100 || len(cfg.S3Buckets) != 1 {
		t.Fatalf("unexpected YAML S3 config: %+v", cfg)
	}
}

func TestValidateS3(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"valid, outside folder", func(*Config) {}, ""},
		{"disabled skips checks", func(c *Config) { c.S3Enabled = false; c.S3Folder = "" }, ""},
		{"port zero", func(c *Config) { c.S3Port = 0 }, "S3_PORT must be a valid"},
		{"port clash", func(c *Config) { c.S3Port = c.Port }, "must differ from PORT"},
		{"folder missing", func(c *Config) { c.S3Folder = "" }, "S3_FOLDER is required"},
		{"keys missing", func(c *Config) { c.S3SecretKey = "" }, "S3_ACCESS_KEY and S3_SECRET_KEY"},
		{"negative ttl", func(c *Config) { c.S3TTL = -time.Second }, "S3_TTL"},
		{"negative max size", func(c *Config) { c.S3MaxObjectSize = -1 }, "S3_MAX_OBJECT_SIZE"},
		{"same as folder", func(c *Config) { c.S3Folder = c.Folder }, "is served by the file server"},
		{"inside folder", func(c *Config) { c.S3Folder = filepath.Join(c.Folder, "cache") }, "is served by the file server"},
		{"dot dir without hiding", func(c *Config) { c.S3Folder = filepath.Join(c.Folder, ".cache") }, "is served by the file server"},
		{"dot dir with hiding", func(c *Config) {
			c.S3Folder = filepath.Join(c.Folder, "data", ".runner-cache", "x")
			c.HideDotFiles = true
		}, ""},
		{"hiding without dot dir", func(c *Config) {
			c.S3Folder = filepath.Join(c.Folder, "cache")
			c.HideDotFiles = true
		}, "is served by the file server"},
		{"sibling with shared prefix", func(c *Config) { c.S3Folder = c.Folder + "-cache" }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validS3Config(t)
			tt.mutate(cfg)
			err := cfg.validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLogValueS3(t *testing.T) {
	cfg := validS3Config(t)
	cfg.S3TTL = time.Hour
	cfg.S3MaxTotalSize = 7
	cfg.S3AccessKey, cfg.S3SecretKey = "s3-access-sentinel", "s3-secret-sentinel"
	s := logged(cfg)
	for _, want := range []string{"config.s3_enabled=true", "config.s3_port=9000", "config.s3_ttl=1h0m0s", "config.s3_max_total_size=7"} {
		if !strings.Contains(s, want) {
			t.Errorf("log missing %q: %s", want, s)
		}
	}
	if strings.Contains(s, cfg.S3AccessKey) || strings.Contains(s, cfg.S3SecretKey) {
		t.Errorf("log must not print the S3 keys: %s", s)
	}
	if strings.Contains(logged(Default()), "s3_port") {
		t.Error("log should omit S3 details when disabled")
	}
}

func TestS3MaxTotalSize(t *testing.T) {
	t.Setenv("S3_MAX_TOTAL_SIZE", "53687091200")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.S3MaxTotalSize != 50<<30 {
		t.Fatalf("S3MaxTotalSize = %d", cfg.S3MaxTotalSize)
	}

	c := validS3Config(t)
	c.S3MaxTotalSize = -1
	if err := c.validate(); err == nil || !strings.Contains(err.Error(), "S3_MAX_TOTAL_SIZE") {
		t.Fatalf("expected S3_MAX_TOTAL_SIZE error, got %v", err)
	}
}

func TestEnableCacheOnly(t *testing.T) {
	cfg := Default()
	if err := cfg.EnableCacheOnly(); err == nil || !strings.Contains(err.Error(), "S3_FOLDER") {
		t.Fatalf("missing S3 settings: expected an S3_FOLDER error, got %v", err)
	}
	if !cfg.CacheOnly || !cfg.S3Enabled {
		t.Fatal("EnableCacheOnly must turn on CacheOnly and S3Enabled")
	}

	// No file server listens or serves, so a shared port and a cache inside FOLDER are fine.
	cfg = Default()
	cfg.S3Port = cfg.Port
	cfg.S3Folder = filepath.Join(cfg.Folder, "cache")
	cfg.S3AccessKey, cfg.S3SecretKey = "ak", "sk"
	if err := cfg.EnableCacheOnly(); err != nil {
		t.Fatalf("cache-only must skip the PORT and FOLDER checks: %v", err)
	}

	cfg.S3Port = 0
	if err := cfg.EnableCacheOnly(); err == nil {
		t.Fatal("S3_PORT 0 must still be rejected")
	}
}
