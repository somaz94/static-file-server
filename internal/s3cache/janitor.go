package s3cache

import (
	"context"
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
	modTime     time.Time
}

// sweep expires objects by upload time (like an S3 lifecycle rule), then evicts the
// oldest objects while the total exceeds MaxTotalSize. Because the runner re-uploads a
// cache after every successful job, oldest-upload-first approximates least-recently-used.
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
			if s.opts.TTL > 0 && now.Sub(info.ModTime()) > s.opts.TTL {
				if s.remove(top, rest) {
					expired++
				}
				return nil
			}
			objects = append(objects, storedObject{top, rest, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})

	evicted := 0
	if s.opts.MaxTotalSize > 0 && total > s.opts.MaxTotalSize {
		sort.Slice(objects, func(i, j int) bool { return objects[i].modTime.Before(objects[j].modTime) })
		for len(objects) > 0 && total > s.opts.MaxTotalSize {
			o := objects[0]
			objects = objects[1:]
			if s.remove(o.bucket, o.key) {
				total -= o.size
				evicted++
			}
		}
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

// remove deletes an object and its metadata sidecar, reporting whether the object was deleted.
func (s *Server) remove(bucket, key string) bool {
	if os.Remove(s.store.dataPath(bucket, key)) != nil {
		return false
	}
	_ = os.Remove(s.store.metaPath(bucket, key))
	return true
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
