// Package server provides HTTP/HTTPS server lifecycle management.
package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/handler"
)

// Run starts the HTTP or HTTPS server based on configuration.
func Run(cfg *config.Config) error {
	h := handler.Build(cfg)

	if cfg.Debug {
		fmt.Fprint(os.Stdout, cfg.Summary())
	}

	addr := cfg.ListenAddr()

	if cfg.HasTLS() {
		return serveTLS(addr, h, cfg)
	}

	fmt.Fprintf(os.Stdout, "Serving %s on HTTP %s\n", cfg.Folder, addr)
	return http.ListenAndServe(addr, h)
}

// serveTLS starts an HTTPS server with TLS configuration.
func serveTLS(addr string, h http.Handler, cfg *config.Config) error {
	tlsCfg := &tls.Config{
		MinVersion: parseTLSVersion(cfg.TLSMinVers),
	}

	srv := &http.Server{
		Addr:      addr,
		Handler:   h,
		TLSConfig: tlsCfg,
	}

	fmt.Fprintf(os.Stdout, "Serving %s on HTTPS %s\n", cfg.Folder, addr)
	return srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
}

// parseTLSVersion converts a version string to a tls.Version constant.
func parseTLSVersion(s string) uint16 {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TLS11":
		return tls.VersionTLS11
	case "TLS12":
		return tls.VersionTLS12
	case "TLS13":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS10
	}
}
