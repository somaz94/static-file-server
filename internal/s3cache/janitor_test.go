package s3cache

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setMtime(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// age sets both the upload time (data file) and the last read (sidecar) of an object.
func age(t *testing.T, s *Server, bucket, key string, at time.Time) {
	t.Helper()
	setMtime(t, s.store.dataPath(bucket, key), at)
	setMtime(t, s.store.metaPath(bucket, key), at)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestSweep(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.TTL = 24 * time.Hour })
	root := s.opts.Folder

	putObject(t, s, "/b-1/old/deep/k", []byte("old"), nil)
	putObject(t, s, "/b-1/new", []byte("new"), nil)
	oldData := s.store.dataPath("b-1", "old/deep/k")
	age(t, s, "b-1", "old/deep/k", testNow.Add(-48*time.Hour))
	age(t, s, "b-1", "new", testNow.Add(-time.Hour))

	staleTmp := filepath.Join(root, tmpDir, "upload-stale")
	freshTmp := filepath.Join(root, tmpDir, "upload-fresh")
	for _, p := range []string{staleTmp, freshTmp} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	setMtime(t, staleTmp, testNow.Add(-25*time.Hour))
	setMtime(t, freshTmp, testNow)

	orphanMeta := s.store.metaPath("b-1", "gone")
	if err := os.WriteFile(orphanMeta, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	s.sweep(testNow)

	checks := []struct {
		path string
		want bool
	}{
		{oldData, false},
		{s.store.metaPath("b-1", "old/deep/k"), false},
		{filepath.Join(root, "b-1", "old"), false}, // emptied directories pruned
		{s.store.dataPath("b-1", "new"), true},
		{s.store.metaPath("b-1", "new"), true},
		{staleTmp, false},
		{freshTmp, true},
		{orphanMeta, false},
		{filepath.Join(root, tmpDir), true},
	}
	for _, c := range checks {
		if got := exists(c.path); got != c.want {
			t.Errorf("%s exists=%v, want %v", c.path, got, c.want)
		}
	}

	// The server keeps working after directories were pruned.
	if w := putObject(t, s, "/b-1/old/deep/k", []byte("again"), nil); w.Code != http.StatusOK {
		t.Fatalf("PUT after sweep: %d %s", w.Code, w.Body.String())
	}
}

func TestSweepEvictsOldestOverCapacity(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.MaxTotalSize = 10 })
	for i, name := range []string{"oldest", "middle", "newest"} {
		putObject(t, s, "/b-1/"+name, []byte("12345"), nil)
		age(t, s, "b-1", name, testNow.Add(time.Duration(i-3)*time.Hour))
	}

	s.sweep(testNow)

	if exists(s.store.dataPath("b-1", "oldest")) || exists(s.store.metaPath("b-1", "oldest")) {
		t.Error("oldest object (and its metadata) should be evicted")
	}
	for _, name := range []string{"middle", "newest"} {
		if !exists(s.store.dataPath("b-1", name)) {
			t.Errorf("%s should be kept", name)
		}
	}
	if got := s.metrics.evictedSize.Load(); got != 1 {
		t.Errorf("evictedSize = %d, want 1", got)
	}
	if s.metrics.objects.Load() != 2 || s.metrics.storedBytes.Load() != 10 {
		t.Errorf("gauges = %d objects / %d bytes, want 2 / 10", s.metrics.objects.Load(), s.metrics.storedBytes.Load())
	}
}

func TestSweepUnderCapacityKeepsEverything(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.MaxTotalSize = 100 })
	putObject(t, s, "/b-1/a", []byte("12345"), nil)
	putObject(t, s, "/b-1/b", []byte("123"), nil)

	s.sweep(testNow)

	if s.metrics.evictedSize.Load() != 0 || s.metrics.objects.Load() != 2 || s.metrics.storedBytes.Load() != 8 {
		t.Errorf("unexpected sweep result: evicted=%d objects=%d bytes=%d",
			s.metrics.evictedSize.Load(), s.metrics.objects.Load(), s.metrics.storedBytes.Load())
	}
}

func TestRunJanitorWithoutTTLStillSweeps(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/k", []byte("abc"), nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.RunJanitor(ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for s.metrics.objects.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if s.metrics.objects.Load() != 1 || s.metrics.storedBytes.Load() != 3 {
		t.Fatalf("first sweep did not update gauges: %d objects / %d bytes", s.metrics.objects.Load(), s.metrics.storedBytes.Load())
	}
	if !exists(s.store.dataPath("b-1", "k")) {
		t.Fatal("TTL=0 must never expire objects")
	}
}

func TestRunJanitorStopsOnCancel(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.TTL = time.Hour })
	putObject(t, s, "/b-1/k", []byte("x"), nil)
	age(t, s, "b-1", "k", testNow.Add(-2*time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.RunJanitor(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for exists(s.store.dataPath("b-1", "k")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if exists(s.store.dataPath("b-1", "k")) {
		t.Fatal("first sweep did not run")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunJanitor did not stop on cancel")
	}
}

func TestReadKeepsCacheAlive(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.TTL = 24 * time.Hour })
	for _, k := range []string{"read", "unread"} {
		putObject(t, s, "/b-1/"+k, []byte("x"), nil)
		age(t, s, "b-1", k, testNow.Add(-48*time.Hour))
	}
	uploaded := testNow.Add(-48 * time.Hour)
	if w := do(t, s, http.MethodGet, "/b-1/read", http.MethodGet, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("GET: %d", w.Code)
	}
	w := do(t, s, http.MethodGet, "/b-1/read", http.MethodGet, nil, nil)
	if got := w.Header().Get("Last-Modified"); got != uploaded.UTC().Format(http.TimeFormat) {
		t.Fatalf("Last-Modified must stay the upload time, got %q", got)
	}

	s.sweep(testNow)
	if !exists(s.store.dataPath("b-1", "read")) {
		t.Error("an object read within the TTL must survive, even if uploaded before it")
	}
	if exists(s.store.dataPath("b-1", "unread")) {
		t.Error("an object neither uploaded nor read within the TTL must expire")
	}
}

func TestTouch(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/k", []byte("x"), nil)
	meta := s.store.metaPath("b-1", "k")
	mtime := func() time.Time {
		fi, err := os.Stat(meta)
		if err != nil {
			t.Fatal(err)
		}
		return fi.ModTime()
	}

	recent := testNow.Add(-30 * time.Minute)
	setMtime(t, meta, recent)
	do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, nil, nil)
	if !mtime().Equal(recent) {
		t.Errorf("a read within touchInterval must not rewrite the sidecar, mtime=%v", mtime())
	}

	// HEAD and the runner's single-byte range probe count as use too.
	for _, h := range []http.Header{nil, {"Range": {"bytes=0-0"}}} {
		setMtime(t, meta, testNow.Add(-2*time.Hour))
		method := http.MethodHead
		if h != nil {
			method = http.MethodGet
		}
		do(t, s, method, "/b-1/k", http.MethodGet, h, nil)
		if !mtime().Equal(testNow) {
			t.Errorf("%s %v: sidecar mtime=%v, want %v", method, h, mtime(), testNow)
		}
	}

	// A missing sidecar is not recreated by a read.
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	do(t, s, http.MethodGet, "/b-1/k", http.MethodGet, nil, nil)
	if exists(meta) {
		t.Error("touch must not create a sidecar")
	}
}

func TestSweepEvictsLeastRecentlyUsed(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.MaxTotalSize = 10 })
	for i, name := range []string{"oldest-but-read", "middle", "newest"} {
		putObject(t, s, "/b-1/"+name, []byte("12345"), nil)
		age(t, s, "b-1", name, testNow.Add(time.Duration(i-3)*time.Hour))
	}
	setMtime(t, s.store.metaPath("b-1", "oldest-but-read"), testNow)

	s.sweep(testNow)
	if exists(s.store.dataPath("b-1", "middle")) {
		t.Error("the least recently used object should be evicted")
	}
	for _, name := range []string{"oldest-but-read", "newest"} {
		if !exists(s.store.dataPath("b-1", name)) {
			t.Errorf("%s should be kept", name)
		}
	}
}

func TestEvictCountsObjectsAnotherReplicaRemoved(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/a", []byte("12345"), nil)
	putObject(t, s, "/b-1/b", []byte("12345"), nil)
	objects := []storedObject{{bucket: "b-1", key: "gone", size: 5}, {bucket: "b-1", key: "a", size: 5}, {bucket: "b-1", key: "b", size: 5}}

	kept, total, evicted := s.evict(objects, 15, 10)
	if evicted != 0 || total != 10 || len(kept) != 2 {
		t.Fatalf("evict = %d kept / %d bytes / %d evicted, want 2 / 10 / 0", len(kept), total, evicted)
	}
	if !exists(s.store.dataPath("b-1", "a")) {
		t.Error("an object already removed by another replica must free its size, not cost the next object")
	}
}

func TestEvictKeepsObjectsItCannotRemove(t *testing.T) {
	s := newTestServer(t, nil)
	putObject(t, s, "/b-1/dir/child", []byte("12345"), nil)
	putObject(t, s, "/b-1/k", []byte("12345"), nil)
	// "dir" is a non-empty directory, so removing it as an object fails.
	objects := []storedObject{{bucket: "b-1", key: "dir", size: 5}, {bucket: "b-1", key: "k", size: 5}}

	kept, total, evicted := s.evict(objects, 10, 5)
	if evicted != 1 || total != 5 || len(kept) != 1 || kept[0].key != "dir" {
		t.Fatalf("evict = %+v / %d bytes / %d evicted", kept, total, evicted)
	}
}
