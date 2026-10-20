// Package config handles configuration loading from environment variables,
// YAML files, and defaults. Priority: env vars > YAML > defaults.
package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

	if err := cfg.loadEnv(); err != nil {
		return nil, fmt.Errorf("env: %w", err)
	}

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

// loadEnv overrides configuration with environment variables when set. A malformed
// typed value is an error: ignoring S3_TTL=7d or S3_MAX_TOTAL_SIZE=50Gi would silently
// disable expiry and the size limit.
func (c *Config) loadEnv() error {
	var e envBinder
	e.bool("CORS", &c.CORS)
	e.bool("DEBUG", &c.Debug)
	e.string("HOST", &c.Host)
	e.uint16("PORT", &c.Port)
	e.string("FOLDER", &c.Folder)
	e.bool("ALLOW_INDEX", &c.AllowIndex)
	e.bool("SHOW_LISTING", &c.ShowListing)
	e.string("URL_PREFIX", &c.URLPrefix)
	e.string("TLS_CERT", &c.TLSCert)
	e.string("TLS_KEY", &c.TLSKey)
	e.string("TLS_MIN_VERS", &c.TLSMinVers)
	if v, ok := os.LookupEnv("REFERRERS"); ok {
		c.Referrers = parseReferrers(v)
	}
	e.string("ACCESS_KEY", &c.AccessKey)
	if v, ok := os.LookupEnv("CUSTOM_HEADERS"); ok {
		c.CustomHeaders = parseHeaders(v)
	}
	e.bool("SPA", &c.SPA)
	e.bool("COMPRESSION", &c.Compression)
	e.bool("HIDE_DOT_FILES", &c.HideDotFiles)
	e.string("LOG_FORMAT", &c.LogFormat)
	e.bool("METRICS", &c.Metrics)
	e.bool("S3_ENABLED", &c.S3Enabled)
	e.uint16("S3_PORT", &c.S3Port)
	e.string("S3_FOLDER", &c.S3Folder)
	e.string("S3_ACCESS_KEY", &c.S3AccessKey)
	e.string("S3_SECRET_KEY", &c.S3SecretKey)
	if v, ok := os.LookupEnv("S3_BUCKETS"); ok {
		c.S3Buckets = parseReferrers(v)
	}
	e.duration("S3_TTL", &c.S3TTL)
	e.int64("S3_MAX_OBJECT_SIZE", &c.S3MaxObjectSize)
	e.int64("S3_MAX_TOTAL_SIZE", &c.S3MaxTotalSize)
	return errors.Join(e.errs...)
}

// envBinder applies environment variables to typed fields and collects parse errors.
// An empty typed value counts as unset, as compose files render ${VAR} for a missing VAR.
type envBinder struct {
	errs []error
}

func (e *envBinder) string(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}

// lookup returns the trimmed value of a typed variable, or false when it is unset or empty.
func (e *envBinder) lookup(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (e *envBinder) fail(key, raw, want string) {
	e.errs = append(e.errs, fmt.Errorf("%s=%q: must be %s", key, raw, want))
}

func (e *envBinder) bool(key string, dst *bool) {
	raw, ok := e.lookup(key)
	if !ok {
		return
	}
	v, valid := parseBool(raw)
	if !valid {
		e.fail(key, raw, "a boolean (true/t/yes/y/1 or false/f/no/n/0)")
		return
	}
	*dst = v
}

func (e *envBinder) uint16(key string, dst *uint16) {
	raw, ok := e.lookup(key)
	if !ok {
		return
	}
	v, err := strconv.ParseUint(raw, 10, 16)
	if err != nil {
		e.fail(key, raw, "a port number (0-65535)")
		return
	}
	*dst = uint16(v)
}

func (e *envBinder) duration(key string, dst *time.Duration) {
	raw, ok := e.lookup(key)
	if !ok {
		return
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		e.fail(key, raw, "a Go duration such as 336h")
		return
	}
	*dst = v
}

func (e *envBinder) int64(key string, dst *int64) {
	raw, ok := e.lookup(key)
	if !ok {
		return
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		e.fail(key, raw, "a whole number of bytes")
		return
	}
	*dst = v
}

// validate checks for configuration errors.
func (c *Config) validate() error {
	if c.Port == 0 {
		return fmt.Errorf("port must be a valid port number (1-65535)")
	}

	if (c.TLSCert == "") != (c.TLSKey == "") {
		return fmt.Errorf("both TLS_CERT and TLS_KEY must be set together")
	}

	if _, ok := tlsVersions[normalizeTLS(c.TLSMinVers)]; c.TLSMinVers != "" && !ok {
		return fmt.Errorf("invalid TLS_MIN_VERS %q: must be TLS11, TLS12, or TLS13", c.TLSMinVers)
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

// tlsVersions maps TLS_MIN_VERS values to crypto/tls versions.
var tlsVersions = map[string]uint16{
	"TLS11": tls.VersionTLS11,
	"TLS12": tls.VersionTLS12,
	"TLS13": tls.VersionTLS13,
}

func normalizeTLS(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// TLSMinVersion returns the crypto/tls minimum version, TLS 1.2 when TLS_MIN_VERS is unset.
func (c *Config) TLSMinVersion() uint16 {
	if v, ok := tlsVersions[normalizeTLS(c.TLSMinVers)]; ok {
		return v
	}
	return tls.VersionTLS12
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

// LogValue implements slog.LogValuer for the debug startup log. Keys are reported only
// as set or unset.
func (c *Config) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Bool("cors", c.CORS),
		slog.Bool("debug", c.Debug),
		slog.String("host", c.Host),
		slog.Int("port", int(c.Port)),
		slog.String("folder", c.Folder),
		slog.Bool("allow_index", c.AllowIndex),
		slog.Bool("show_listing", c.ShowListing),
		slog.String("url_prefix", c.URLPrefix),
		slog.Bool("tls", c.HasTLS()),
		slog.String("tls_min_vers", c.TLSMinVers),
		slog.Any("referrers", c.Referrers),
		slog.Bool("access_key", c.AccessKey != ""),
		// Header names only: a value can carry a token.
		slog.Any("custom_headers", slices.Sorted(maps.Keys(c.CustomHeaders))),
		slog.Bool("spa", c.SPA),
		slog.Bool("compression", c.Compression),
		slog.Bool("hide_dot_files", c.HideDotFiles),
		slog.String("log_format", c.LogFormat),
		slog.Bool("metrics", c.Metrics),
		slog.Bool("s3_enabled", c.S3Enabled),
	}
	if c.S3Enabled {
		attrs = append(attrs,
			slog.Bool("cache_only", c.CacheOnly),
			slog.Int("s3_port", int(c.S3Port)),
			slog.String("s3_folder", c.S3Folder),
			slog.Any("s3_buckets", c.S3Buckets),
			slog.Duration("s3_ttl", c.S3TTL),
			slog.Int64("s3_max_object_size", c.S3MaxObjectSize),
			slog.Int64("s3_max_total_size", c.S3MaxTotalSize),
		)
	}
	return slog.GroupValue(attrs...)
}

// parseBool accepts 1, true, t, yes, y and 0, false, f, no, n, case-insensitively.
func parseBool(raw string) (value, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "t", "yes", "y":
		return true, true
	case "0", "false", "f", "no", "n":
		return false, true
	default:
		return false, false
	}
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
