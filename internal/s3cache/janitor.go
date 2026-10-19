package s3cache

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxSweepInterval = time.Hour
	// Abandoned uploads are kept long enough to never race a slow in-flight PUT.
	staleTmpAge = 24 * time.Hour
)

// RunJanitor deletes objects older than opts.TTL until ctx is cancelled. It is a no-op when TTL is 0.
func (s *Server) RunJanitor(ctx context.Context) {
	if s.opts.TTL <= 0 {
		return
	}
	interval := min(s.opts.TTL, maxSweepInterval)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		s.sweep(s.now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sweep removes expired objects with their metadata, stale temp files and orphaned metadata,
// then prunes empty directories. Expiry is by upload time, like an S3 lifecycle rule.
func (s *Server) sweep(now time.Time) {
	root := s.store.root
	removed := 0

	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		top, rest, _ := strings.Cut(filepath.ToSlash(rel), "/")
		info, err := d.Info()
		if err != nil {
			return nil
		}

		switch top {
		case tmpDir:
			if now.Sub(info.ModTime()) > staleTmpAge {
				_ = os.Remove(p)
			}
		case metaDir:
			bucket, key, _ := strings.Cut(strings.TrimSuffix(rest, ".json"), "/")
			if _, err := os.Stat(s.store.dataPath(bucket, key)); os.IsNotExist(err) {
				_ = os.Remove(p)
			}
		default:
			if now.Sub(info.ModTime()) > s.opts.TTL {
				if os.Remove(p) == nil {
					_ = os.Remove(s.store.metaPath(top, rest))
					removed++
				}
			}
		}
		return nil
	})

	pruneEmptyDirs(root)
	if removed > 0 {
		log.Printf("s3cache: janitor removed %d expired object(s)", removed)
	}
}

// pruneEmptyDirs removes empty directories below root, deepest first, keeping root and tmpDir.
func pruneEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != root && p != filepath.Join(root, tmpDir) {
			dirs = append(dirs, p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i]) // fails, harmlessly, on non-empty directories
	}
}
