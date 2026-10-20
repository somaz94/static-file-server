// Package config handles configuration loading from environment variables,
// YAML files, and defaults. Priority: env vars > YAML > defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all server configuration.
type Config struct {
	CORS          bool              `yaml:"cors"`
	Debug         bool              `yaml:"debug"`
	Host          string            `yaml:"host"`
	Port          uint16            `yaml:"port"`
	Folder        string            `yaml:"folder"`
	AllowIndex    bool              `yaml:"allow-index"`
	ShowListing   bool              `yaml:"show-listing"`
	URLPrefix     string            `yaml:"url-prefix"`
	TLSCert       string            `yaml:"tls-cert"`
	TLSKey        string            `yaml:"tls-key"`
	TLSMinVers    string            `yaml:"tls-min-vers"`
	Referrers     []string          `yaml:"referrers"`
	AccessKey     string            `yaml:"access-key"`
	CustomHeaders map[string]string `yaml:"custom-headers"`
	SPA           bool              `yaml:"spa"`
	Compression   bool              `yaml:"compression"`
	HideDotFiles  bool              `yaml:"hide-dot-files"`
	LogFormat     string            `yaml:"log-format"`
	Metrics       bool              `yaml:"metrics"`

	S3Enabled       bool          `yaml:"s3-enabled"`
	S3Port          uint16        `yaml:"s3-port"`
	S3Folder        string        `yaml:"s3-folder"`
	S3AccessKey     string        `yaml:"s3-access-key"`
	S3SecretKey     string        `yaml:"s3-secret-key"`
	S3Buckets       []string      `yaml:"s3-buckets"`
	S3TTL           time.Duration `yaml:"s3-ttl"`
	S3MaxObjectSize int64         `yaml:"s3-max-object-size"`
	S3MaxTotalSize  int64         `yaml:"s3-max-total-size"`

	// CacheOnly runs the S3 cache endpoint without the file server; set by the cache subcommand.
	CacheOnly bool `yaml:"-"`
}

// Default returns a Config with default values matching halverneus behavior.
func Default() *Config {
	return &Config{
		CORS:          false,
		Debug:         false,
		Host:          "",
		Port:          8080,
		Folder:        "/web",
		AllowIndex:    true,
		ShowListing:   true,
		URLPrefix:     "",
		TLSCert:       "",
		TLSKey:        "",
		TLSMinVers:    "",
		Referrers:     nil,
		AccessKey:     "",
		CustomHeaders: nil,
		SPA:           false,
		Compression:   false,
		HideDotFiles:  false,
		LogFormat:     "text",
		Metrics:       false,
		S3Port:        9000,
	}
}

// Load creates a Config by applying defaults, then YAML overrides, then env var
// overrides. An empty configFile skips YAML loading.
func Load(configFile string) (*Config, error) {
	cfg := Default()

	if configFile != "" {
		if err := cfg.loadYAML(configFile); err != nil {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
	}

	cfg.loadEnv()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// loadYAML reads and applies a YAML configuration file.
func (c *Config) loadYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

// loadEnv overrides configuration with environment variables when set.
func (c *Config) loadEnv() {
	if v, ok := lookupEnvBool("CORS"); ok {
		c.CORS = v
	}
	if v, ok := lookupEnvBool("DEBUG"); ok {
		c.Debug = v
	}
	if v, ok := os.LookupEnv("HOST"); ok {
		c.Host = v
	}
	if v, ok := lookupEnvUint16("PORT"); ok {
		c.Port = v
	}
	if v, ok := os.LookupEnv("FOLDER"); ok {
		c.Folder = v
	}
	if v, ok := lookupEnvBool("ALLOW_INDEX"); ok {
		c.AllowIndex = v
	}
	if v, ok := lookupEnvBool("SHOW_LISTING"); ok {
		c.ShowListing = v
	}
	if v, ok := os.LookupEnv("URL_PREFIX"); ok {
		c.URLPrefix = v
	}
	if v, ok := os.LookupEnv("TLS_CERT"); ok {
		c.TLSCert = v
	}
	if v, ok := os.LookupEnv("TLS_KEY"); ok {
		c.TLSKey = v
	}
	if v, ok := os.LookupEnv("TLS_MIN_VERS"); ok {
		c.TLSMinVers = v
	}
	if v, ok := os.LookupEnv("REFERRERS"); ok {
		c.Referrers = parseReferrers(v)
	}
	if v, ok := os.LookupEnv("ACCESS_KEY"); ok {
		c.AccessKey = v
	}
	if v, ok := os.LookupEnv("CUSTOM_HEADERS"); ok {
		c.CustomHeaders = parseHeaders(v)
	}
	if v, ok := lookupEnvBool("SPA"); ok {
		c.SPA = v
	}
	if v, ok := lookupEnvBool("COMPRESSION"); ok {
		c.Compression = v
	}
	if v, ok := lookupEnvBool("HIDE_DOT_FILES"); ok {
		c.HideDotFiles = v
	}
	if v, ok := os.LookupEnv("LOG_FORMAT"); ok {
		c.LogFormat = v
	}
	if v, ok := lookupEnvBool("METRICS"); ok {
		c.Metrics = v
	}
	if v, ok := lookupEnvBool("S3_ENABLED"); ok {
		c.S3Enabled = v
	}
	if v, ok := lookupEnvUint16("S3_PORT"); ok {
		c.S3Port = v
	}
	if v, ok := os.LookupEnv("S3_FOLDER"); ok {
		c.S3Folder = v
	}
	if v, ok := os.LookupEnv("S3_ACCESS_KEY"); ok {
		c.S3AccessKey = v
	}
	if v, ok := os.LookupEnv("S3_SECRET_KEY"); ok {
		c.S3SecretKey = v
	}
	if v, ok := os.LookupEnv("S3_BUCKETS"); ok {
		c.S3Buckets = parseReferrers(v)
	}
	if v, ok := os.LookupEnv("S3_TTL"); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			c.S3TTL = d
		}
	}
	if v, ok := os.LookupEnv("S3_MAX_OBJECT_SIZE"); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			c.S3MaxObjectSize = n
		}
	}
	if v, ok := os.LookupEnv("S3_MAX_TOTAL_SIZE"); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			c.S3MaxTotalSize = n
		}
	}
}

// validate checks for configuration errors.
func (c *Config) validate() error {
	if c.Port == 0 {
		return fmt.Errorf("port must be a valid port number (1-65535)")
	}

	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("both TLS_CERT and TLS_KEY must be set together")
	}

	if c.TLSMinVers != "" {
		switch strings.ToUpper(strings.TrimSpace(c.TLSMinVers)) {
		case "TLS11", "TLS12", "TLS13":
			// valid
		default:
			return fmt.Errorf("invalid TLS_MIN_VERS %q: must be TLS11, TLS12, or TLS13", c.TLSMinVers)
		}
	}

	if c.URLPrefix != "" {
		if !strings.HasPrefix(c.URLPrefix, "/") {
			return fmt.Errorf("URL_PREFIX must start with /")
		}
		if strings.HasSuffix(c.URLPrefix, "/") {
			return fmt.Errorf("URL_PREFIX must not end with /")
		}
	}

	switch strings.ToLower(c.LogFormat) {
	case "text", "json":
		c.LogFormat = strings.ToLower(c.LogFormat)
	default:
		return fmt.Errorf("LOG_FORMAT must be \"text\" or \"json\", got %q", c.LogFormat)
	}

	if c.SPA && c.ShowListing {
		return fmt.Errorf("SPA mode is incompatible with SHOW_LISTING=true")
	}

	if c.S3Enabled {
		if err := c.validateS3(); err != nil {
			return err
		}
	}

	return nil
}

// EnableCacheOnly switches to serving only the S3 cache endpoint, so the S3 settings
// become required whether or not S3_ENABLED was set.
func (c *Config) EnableCacheOnly() error {
	c.CacheOnly = true
	c.S3Enabled = true
	return c.validateS3()
}

// validateS3 checks the S3 cache endpoint settings. The checks against PORT and FOLDER
// are skipped in cache-only mode, where no file server listens or serves.
func (c *Config) validateS3() error {
	switch {
	case c.S3Port == 0:
		return fmt.Errorf("S3_PORT must be a valid port number (1-65535)")
	case !c.CacheOnly && c.S3Port == c.Port:
		return fmt.Errorf("S3_PORT must differ from PORT")
	case c.S3Folder == "":
		return fmt.Errorf("S3_FOLDER is required for the S3 cache endpoint")
	case c.S3AccessKey == "" || c.S3SecretKey == "":
		return fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required for the S3 cache endpoint")
	case c.S3TTL < 0:
		return fmt.Errorf("S3_TTL must not be negative")
	case c.S3MaxObjectSize < 0:
		return fmt.Errorf("S3_MAX_OBJECT_SIZE must not be negative")
	case c.S3MaxTotalSize < 0:
		return fmt.Errorf("S3_MAX_TOTAL_SIZE must not be negative")
	}
	if c.CacheOnly {
		return nil
	}
	return c.checkS3FolderNotServed()
}

// checkS3FolderNotServed rejects an S3_FOLDER the file server would expose: cache
// archives can hold build secrets. Inside FOLDER is allowed only below a dot
// directory with HIDE_DOT_FILES=true, which blocks serving it.
func (c *Config) checkS3FolderNotServed() error {
	served, err := filepath.Abs(c.Folder)
	if err != nil {
		return fmt.Errorf("resolving FOLDER: %w", err)
	}
	s3, err := filepath.Abs(c.S3Folder)
	if err != nil {
		return fmt.Errorf("resolving S3_FOLDER: %w", err)
	}
	rel, err := filepath.Rel(served, s3)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	if c.HideDotFiles {
		for _, seg := range strings.Split(rel, string(filepath.Separator)) {
			if strings.HasPrefix(seg, ".") && seg != "." {
				return nil
			}
		}
	}
	return fmt.Errorf("S3_FOLDER %q is served by the file server; move it outside FOLDER or under a dot directory with HIDE_DOT_FILES=true", c.S3Folder)
}

// HasTLS returns true if TLS is configured.
func (c *Config) HasTLS() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}

// S3ListenAddr returns the host:port listen address of the S3 cache endpoint.
func (c *Config) S3ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.S3Port)
}

// ListenAddr returns the host:port listen address.
func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Summary returns a human-readable configuration summary for debug output.
func (c *Config) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration:\n")
	fmt.Fprintf(&b, "  CORS:         %t\n", c.CORS)
	fmt.Fprintf(&b, "  Debug:        %t\n", c.Debug)
	fmt.Fprintf(&b, "  Host:         %q\n", c.Host)
	fmt.Fprintf(&b, "  Port:         %d\n", c.Port)
	fmt.Fprintf(&b, "  Folder:       %s\n", c.Folder)
	fmt.Fprintf(&b, "  AllowIndex:   %t\n", c.AllowIndex)
	fmt.Fprintf(&b, "  ShowListing:  %t\n", c.ShowListing)
	fmt.Fprintf(&b, "  URLPrefix:    %q\n", c.URLPrefix)
	fmt.Fprintf(&b, "  TLS:          %t\n", c.HasTLS())
	fmt.Fprintf(&b, "  TLSMinVers:   %q\n", c.TLSMinVers)
	fmt.Fprintf(&b, "  Referrers:    %v\n", c.Referrers)
	fmt.Fprintf(&b, "  AccessKey:    %t\n", c.AccessKey != "")
	fmt.Fprintf(&b, "  CustomHeaders: %v\n", c.CustomHeaders)
	fmt.Fprintf(&b, "  SPA:          %t\n", c.SPA)
	fmt.Fprintf(&b, "  Compression:  %t\n", c.Compression)
	fmt.Fprintf(&b, "  HideDotFiles: %t\n", c.HideDotFiles)
	fmt.Fprintf(&b, "  LogFormat:    %s\n", c.LogFormat)
	fmt.Fprintf(&b, "  Metrics:      %t\n", c.Metrics)
	fmt.Fprintf(&b, "  S3Enabled:    %t\n", c.S3Enabled)
	if c.S3Enabled {
		fmt.Fprintf(&b, "  S3Port:       %d\n", c.S3Port)
		fmt.Fprintf(&b, "  S3Folder:     %s\n", c.S3Folder)
		fmt.Fprintf(&b, "  S3Buckets:    %v\n", c.S3Buckets)
		fmt.Fprintf(&b, "  S3TTL:        %s\n", c.S3TTL)
		fmt.Fprintf(&b, "  S3MaxObjectSize: %d\n", c.S3MaxObjectSize)
		fmt.Fprintf(&b, "  S3MaxTotalSize:  %d\n", c.S3MaxTotalSize)
	}
	return b.String()
}

// lookupEnvBool reads a boolean environment variable.
// Accepts: 1, true, t, yes, y (true) and 0, false, f, no, n (false).
func lookupEnvBool(key string) (bool, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "t", "yes", "y":
		return true, true
	case "0", "false", "f", "no", "n":
		return false, true
	default:
		return false, false
	}
}

// lookupEnvUint16 reads a uint16 environment variable.
func lookupEnvUint16(key string) (uint16, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}

// parseReferrers splits a comma-separated string into a referrer list.
func parseReferrers(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}

// parseHeaders parses a comma-separated "Key:Value,Key2:Value2" string into a map.
func parseHeaders(s string) map[string]string {
	if s == "" {
		return nil
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
