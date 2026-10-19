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
	setMtime(t, oldData, testNow.Add(-48*time.Hour))
	setMtime(t, s.store.dataPath("b-1", "new"), testNow.Add(-time.Hour))

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

func TestRunJanitorDisabled(t *testing.T) {
	s := newTestServer(t, nil)
	done := make(chan struct{})
	go func() {
		s.RunJanitor(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunJanitor with TTL=0 should return immediately")
	}
}

func TestRunJanitorStopsOnCancel(t *testing.T) {
	s := newTestServer(t, func(o *Options) { o.TTL = time.Hour })
	putObject(t, s, "/b-1/k", []byte("x"), nil)
	setMtime(t, s.store.dataPath("b-1", "k"), testNow.Add(-2*time.Hour))

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
