package s3cache

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxSweepInterval = 10 * time.Minute
	// Abandoned uploads are kept long enough to never race a slow in-flight PUT.
	staleTmpAge = 24 * time.Hour
)

// RunJanitor sweeps the store until ctx is cancelled: TTL and total-size eviction,
// stale temp files, orphaned metadata, and the object gauges.
func (s *Server) RunJanitor(ctx context.Context) {
	interval := maxSweepInterval
	if s.opts.TTL > 0 {
		interval = min(s.opts.TTL, maxSweepInterval)
	}
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

type storedObject struct {
	bucket, key string
	size        int64
	lastUsed    time.Time
}

// sweep expires objects unused for longer than the TTL (upload or read, see store.touch), then
// evicts the least recently used objects while the total exceeds MaxTotalSize.
func (s *Server) sweep(now time.Time) {
	root := s.store.root
	var objects []storedObject
	var total int64
	expired := 0

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
			lastUsed := s.store.lastUsed(top, rest, info.ModTime())
			if s.opts.TTL > 0 && now.Sub(lastUsed) > s.opts.TTL {
				if s.remove(top, rest) == nil {
					expired++
				}
				return nil
			}
			objects = append(objects, storedObject{top, rest, info.Size(), lastUsed})
			total += info.Size()
		}
		return nil
	})

	evicted := 0
	if s.opts.MaxTotalSize > 0 && total > s.opts.MaxTotalSize {
		sort.Slice(objects, func(i, j int) bool { return objects[i].lastUsed.Before(objects[j].lastUsed) })
		objects, total, evicted = s.evict(objects, total, s.opts.MaxTotalSize)
	}

	pruneEmptyDirs(root)
	s.metrics.evictedTTL.Add(int64(expired))
	s.metrics.evictedSize.Add(int64(evicted))
	s.metrics.objects.Store(int64(len(objects)))
	s.metrics.storedBytes.Store(total)
	if expired > 0 || evicted > 0 {
		log.Printf("s3cache: janitor removed %d expired and %d over-capacity object(s); %d object(s), %d byte(s) remain",
			expired, evicted, len(objects), total)
	}
}

// evict removes objects, sorted least recently used first, until total fits in limit.
// It returns the objects still stored, their total size and how many it deleted.
func (s *Server) evict(objects []storedObject, total, limit int64) ([]storedObject, int64, int) {
	var kept []storedObject
	evicted := 0
	for i, o := range objects {
		if total <= limit {
			return append(kept, objects[i:]...), total, evicted
		}
		switch err := s.remove(o.bucket, o.key); {
		case err == nil:
			total -= o.size
			evicted++
		case errors.Is(err, fs.ErrNotExist):
			// Another replica sharing the volume evicted it first. Still counting its size
			// would make this sweep delete objects the cap no longer requires.
			total -= o.size
		default:
			kept = append(kept, o)
		}
	}
	return kept, total, evicted
}

// remove deletes an object and its metadata sidecar.
func (s *Server) remove(bucket, key string) error {
	if err := os.Remove(s.store.dataPath(bucket, key)); err != nil {
		return err
	}
	_ = os.Remove(s.store.metaPath(bucket, key))
	return nil
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
