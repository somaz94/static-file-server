// Package server provides HTTP/HTTPS server lifecycle management.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/handler"
)

const (
	readTimeout  = 10 * time.Second
	writeTimeout = 30 * time.Second
	idleTimeout  = 120 * time.Second
	shutdownWait = 10 * time.Second
)

// Run starts the HTTP or HTTPS server based on configuration.
func Run(cfg *config.Config) error {
	h := handler.Build(cfg)

	if cfg.Debug {
		fmt.Fprint(os.Stdout, cfg.Summary())
	}

	addr := cfg.ListenAddr()

	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	if cfg.HasTLS() {
		srv.TLSConfig = &tls.Config{
			MinVersion: parseTLSVersion(cfg.TLSMinVers),
		}
	}

	return listenAndServe(srv, cfg)
}

// listenAndServe starts the server and handles graceful shutdown on SIGTERM/SIGINT.
func listenAndServe(srv *http.Server, cfg *config.Config) error {
	errCh := make(chan error, 1)

	go func() {
		if cfg.HasTLS() {
			fmt.Fprintf(os.Stdout, "Serving %s on HTTPS %s\n", cfg.Folder, srv.Addr)
			errCh <- srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			fmt.Fprintf(os.Stdout, "Serving %s on HTTP %s\n", cfg.Folder, srv.Addr)
			errCh <- srv.ListenAndServe()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		fmt.Fprintf(os.Stdout, "\nReceived %s, shutting down gracefully...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		fmt.Fprintln(os.Stdout, "Server stopped")
		return nil
	}
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
		return tls.VersionTLS12
	}
}
