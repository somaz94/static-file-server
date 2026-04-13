package server

import (
	"crypto/tls"
	"testing"
)

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		{"TLS10", tls.VersionTLS10},
		{"TLS11", tls.VersionTLS11},
		{"TLS12", tls.VersionTLS12},
		{"TLS13", tls.VersionTLS13},
		{"tls12", tls.VersionTLS12}, // case insensitive
		{"  TLS13  ", tls.VersionTLS13}, // whitespace trimmed
		{"", tls.VersionTLS10},       // empty defaults to TLS10
		{"invalid", tls.VersionTLS10}, // invalid defaults to TLS10
		{"TLS99", tls.VersionTLS10},   // unknown defaults to TLS10
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
