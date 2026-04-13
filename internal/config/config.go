// Package config handles configuration loading from environment variables,
// YAML files, and defaults. Priority: env vars > YAML > defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all server configuration.
type Config struct {
	CORS        bool     `yaml:"cors"`
	Debug       bool     `yaml:"debug"`
	Host        string   `yaml:"host"`
	Port        uint16   `yaml:"port"`
	Folder      string   `yaml:"folder"`
	AllowIndex  bool     `yaml:"allow-index"`
	ShowListing bool     `yaml:"show-listing"`
	URLPrefix   string   `yaml:"url-prefix"`
	TLSCert     string   `yaml:"tls-cert"`
	TLSKey      string   `yaml:"tls-key"`
	TLSMinVers  string   `yaml:"tls-min-vers"`
	Referrers   []string `yaml:"referrers"`
	AccessKey   string   `yaml:"access-key"`
}

// Default returns a Config with default values matching halverneus behavior.
func Default() *Config {
	return &Config{
		CORS:        false,
		Debug:       false,
		Host:        "",
		Port:        8080,
		Folder:      "/web",
		AllowIndex:  true,
		ShowListing: true,
		URLPrefix:   "",
		TLSCert:     "",
		TLSKey:      "",
		TLSMinVers:  "",
		Referrers:   nil,
		AccessKey:   "",
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
		case "TLS10", "TLS11", "TLS12", "TLS13":
			// valid
		default:
			return fmt.Errorf("invalid TLS_MIN_VERS %q: must be TLS10, TLS11, TLS12, or TLS13", c.TLSMinVers)
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

	return nil
}

// HasTLS returns true if TLS is configured.
func (c *Config) HasTLS() bool {
	return c.TLSCert != "" && c.TLSKey != ""
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
