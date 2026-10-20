package s3cache

import (
	"crypto/md5" //nolint:gosec // S3 ETag format, not a security boundary
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	metaDir = ".meta"
	tmpDir  = ".tmp"

	// touchInterval bounds last-use writes to one per object per hour, which is plenty for day-scale TTLs.
	touchInterval = time.Hour
)

// Bucket names follow the S3 rules, so they can never start with '.' and clash with metaDir / tmpDir.
var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

var (
	errNotFound    = errors.New("object not found")
	errConflict    = errors.New("key conflicts with an existing path")
	errShortBody   = errors.New("body shorter than Content-Length")
	errInvalidName = errors.New("invalid bucket or key")
)

// objectMeta is stored beside each object under metaDir.
type objectMeta struct {
	ETag        string            `json:"etag"`
	ContentType string            `json:"content_type,omitempty"`
	UserMeta    map[string]string `json:"user_meta,omitempty"`
}

type store struct {
	root string
}

func validName(bucket, key string) bool {
	if !bucketNameRe.MatchString(bucket) || key == "" || strings.ContainsRune(key, 0) {
		return false
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func (s *store) dataPath(bucket, key string) string {
	return filepath.Join(s.root, bucket, filepath.FromSlash(key))
}

func (s *store) metaPath(bucket, key string) string {
	return filepath.Join(s.root, metaDir, bucket, filepath.FromSlash(key)+".json")
}

// open returns the object file, its metadata (zero value if the sidecar is missing) and stat info.
func (s *store) open(bucket, key string) (*os.File, objectMeta, os.FileInfo, error) {
	var meta objectMeta
	if !validName(bucket, key) {
		return nil, meta, nil, errInvalidName
	}
	f, err := os.Open(s.dataPath(bucket, key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil, meta, nil, errNotFound
		}
		return nil, meta, nil, err
	}
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		_ = f.Close()
		if err == nil {
			err = errNotFound
		}
		return nil, meta, nil, err
	}
	if raw, err := os.ReadFile(s.metaPath(bucket, key)); err == nil {
		_ = json.Unmarshal(raw, &meta)
	}
	return f, meta, fi, nil
}

// touch records a read as the object's last use in its sidecar's mtime. The runner skips
// re-uploading an unchanged cache, so the data file's mtime alone would expire caches in daily
// use. The data file is left alone so Last-Modified keeps meaning upload time.
func (s *store) touch(bucket, key string, now time.Time) {
	p := s.metaPath(bucket, key)
	if fi, err := os.Stat(p); err == nil && now.Sub(fi.ModTime()) >= touchInterval {
		_ = os.Chtimes(p, now, now)
	}
}

// lastUsed is the later of the upload time and the last recorded read.
func (s *store) lastUsed(bucket, key string, uploaded time.Time) time.Time {
	if fi, err := os.Stat(s.metaPath(bucket, key)); err == nil && fi.ModTime().After(uploaded) {
		return fi.ModTime()
	}
	return uploaded
}

// put streams exactly size bytes from body into the object, replacing it atomically.
func (s *store) put(bucket, key string, body io.Reader, size int64, meta objectMeta) (string, error) {
	if !validName(bucket, key) {
		return "", errInvalidName
	}
	tmpRoot := filepath.Join(s.root, tmpDir)
	if err := os.MkdirAll(tmpRoot, 0o750); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(tmpRoot, "upload-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	h := md5.New() //nolint:gosec // S3 ETag format
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(body, size))
	if err == nil && n != size {
		err = errShortBody
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}

	dst := s.dataPath(bucket, key)
	if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
		return "", errConflict
	}
	if err := place(tmpName, dst); err != nil {
		return "", err
	}

	meta.ETag = `"` + hex.EncodeToString(h.Sum(nil)) + `"`
	if err := s.writeMeta(bucket, key, meta); err != nil {
		return "", fmt.Errorf("writing metadata: %w", err)
	}
	return meta.ETag, nil
}

func (s *store) writeMeta(bucket, key string, meta objectMeta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	dst := s.metaPath(bucket, key)
	tmp, err := os.CreateTemp(filepath.Join(s.root, tmpDir), "meta-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	_, err = tmp.Write(raw)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return place(tmpName, dst)
}

// place renames src to dst, retrying once if the janitor pruned dst's freshly created parent.
func place(src, dst string) error {
	var err error
	for range 2 {
		if err = mkdirParent(dst); err != nil {
			return err
		}
		if err = os.Rename(src, dst); !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return err
}

// mkdirParent creates dst's parent directory, mapping "a path component is a file" to errConflict.
func mkdirParent(dst string) error {
	err := os.MkdirAll(filepath.Dir(dst), 0o750)
	if err != nil && (errors.Is(err, syscall.ENOTDIR) || errors.Is(err, os.ErrExist)) {
		return errConflict
	}
	return err
}
