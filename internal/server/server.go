// Package server provides HTTP/HTTPS server lifecycle management.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/handler"
	"github.com/somaz94/static-file-server/internal/s3cache"
)

const (
	readTimeout  = 10 * time.Second
	idleTimeout  = 120 * time.Second
	shutdownWait = 10 * time.Second

	// Cache archives can be gigabytes, so the S3 listener bounds only the header read.
	s3ReadHeaderTimeout = 10 * time.Second
)

// NewLogger returns the process logger: LOG_FORMAT picks text or JSON, DEBUG enables
// the debug level (request logs and the startup configuration).
func NewLogger(w io.Writer, cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfg.Debug {
		opts.Level = slog.LevelDebug
	}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

// namedServer pairs a server with the label used in lifecycle logs.
type namedServer struct {
	srv   *http.Server
	label string
}

// Run starts the file server and, when enabled, the S3 cache endpoint, and shuts them
// down gracefully when ctx is cancelled. In cache-only mode only the S3 cache endpoint
// starts.
func Run(ctx context.Context, cfg *config.Config) error {
	return run(ctx, cfg, NewLogger(os.Stderr, cfg))
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Debug("configuration", "config", cfg)
	errorLog := slog.NewLogLogger(logger.Handler(), slog.LevelWarn)

	var servers []namedServer
	if !cfg.CacheOnly {
		servers = append(servers, namedServer{
			srv: &http.Server{
				Addr:    cfg.ListenAddr(),
				Handler: handler.Build(cfg, logger),
				// No WriteTimeout: it caps the whole response, so it cut off any download slower than its value.
				ReadHeaderTimeout: readTimeout,
				ReadTimeout:       readTimeout,
				IdleTimeout:       idleTimeout,
				ErrorLog:          errorLog,
			},
			label: cfg.Folder,
		})
	}

	// The janitor stops on return, not on ctx, so it never sweeps a store that is still serving.
	janitorCtx, stopJanitor := context.WithCancel(context.Background())
	var janitor sync.WaitGroup
	defer func() {
		stopJanitor()
		janitor.Wait()
	}()

	if cfg.S3Enabled {
		s3 := s3cache.New(s3cache.Options{
			Folder:        cfg.S3Folder,
			AccessKey:     cfg.S3AccessKey,
			SecretKey:     cfg.S3SecretKey,
			Buckets:       cfg.S3Buckets,
			MaxObjectSize: cfg.S3MaxObjectSize,
			MaxTotalSize:  cfg.S3MaxTotalSize,
			TTL:           cfg.S3TTL,
			Metrics:       cfg.Metrics,
			Logger:        logger,
		})
		janitor.Go(func() { s3.RunJanitor(janitorCtx) })
		servers = append(servers, namedServer{
			srv: &http.Server{
				Addr:              cfg.S3ListenAddr(),
				Handler:           s3,
				ReadHeaderTimeout: s3ReadHeaderTimeout,
				IdleTimeout:       idleTimeout,
				ErrorLog:          errorLog,
			},
			label: "S3 cache " + cfg.S3Folder,
		})
	}

	if len(servers) == 0 {
		return errors.New("nothing to serve: cache-only mode requires the S3 cache endpoint")
	}

	if cfg.HasTLS() {
		for _, s := range servers {
			s.srv.TLSConfig = &tls.Config{
				MinVersion: parseTLSVersion(cfg.TLSMinVers),
			}
		}
	}

	return listenAndServe(ctx, cfg, logger, servers)
}

// listenAndServe starts every server and shuts all of them down gracefully when ctx is
// cancelled, or when any one of them fails.
func listenAndServe(ctx context.Context, cfg *config.Config, logger *slog.Logger, servers []namedServer) error {
	errCh := make(chan error, len(servers))

	for _, s := range servers {
		go func() {
			var err error
			if cfg.HasTLS() {
				logger.Info("serving", "listener", s.label, "scheme", "https", "addr", s.srv.Addr)
				err = s.srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
			} else {
				logger.Info("serving", "listener", s.label, "scheme", "http", "addr", s.srv.Addr)
				err = s.srv.ListenAndServe()
			}
			errCh <- fmt.Errorf("%s: %w", s.label, err)
		}()
	}

	select {
	case err := <-errCh:
		_ = shutdownAll(servers)
		return err
	case <-ctx.Done():
		logger.Info("shutting down", "cause", context.Cause(ctx))
		if err := shutdownAll(servers); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		logger.Info("server stopped")
		return nil
	}
}

// shutdownAll shuts the servers down in parallel so they share one shutdownWait budget.
func shutdownAll(servers []namedServer) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	errs := make([]error, len(servers))
	var wg sync.WaitGroup
	for i, s := range servers {
		wg.Go(func() {
			if err := s.srv.Shutdown(ctx); err != nil {
				errs[i] = fmt.Errorf("%s: %w", s.label, err)
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
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
