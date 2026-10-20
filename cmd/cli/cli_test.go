package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/version"
)

func TestExecuteHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := Execute()
	if err != nil {
		t.Fatalf("Execute() with --help returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "static-file-server") {
		t.Error("help output should contain 'static-file-server'")
	}
}

func TestVersionSubcommand(t *testing.T) {
	rootCmd.SetArgs([]string{"version"})
	var buf bytes.Buffer
	// Cobra's version command uses fmt.Println, so we redirect cmd output
	// and also set the version command's output.
	rootCmd.SetOut(&buf)
	versionCmd.SetOut(&buf)
	// Override Run to use cmd.Println so output is captured.
	origRun := versionCmd.Run
	versionCmd.Run = func(cmd *cobra.Command, args []string) {
		cmd.Println(version.String())
	}
	defer func() { versionCmd.Run = origRun }()

	err := Execute()
	if err != nil {
		t.Fatalf("Execute() version returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "static-file-server") {
		t.Errorf("version output should contain 'static-file-server', got %q", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("version output should contain 'commit:', got %q", output)
	}
}

func TestInvalidConfig(t *testing.T) {
	// Use a fresh command to avoid state leaking from prior tests.
	rootCmd.SetArgs([]string{"--config", "/nonexistent/path/config.yaml"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	// Reset cfgFile to avoid stale flag values.
	cfgFile = "/nonexistent/path/config.yaml"
	defer func() { cfgFile = "" }()

	err := rootCmd.RunE(rootCmd, []string{})
	if err == nil {
		t.Error("runServe with invalid config path should return error")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Errorf("error should mention config, got: %v", err)
	}
}

func TestCacheSubcommandRequiresS3Settings(t *testing.T) {
	t.Setenv("S3_FOLDER", "")
	rootCmd.SetArgs([]string{"cache"})
	err := Execute()
	if err == nil || !strings.Contains(err.Error(), "S3_FOLDER") {
		t.Fatalf("expected an S3_FOLDER error, got %v", err)
	}
}

func TestCacheSubcommandInvalidConfig(t *testing.T) {
	rootCmd.SetArgs([]string{"cache", "--config", "/nonexistent/path/config.yaml"})
	if err := Execute(); err == nil {
		t.Fatal("expected an error for a missing config file")
	}
}

func TestServeStopsWhenContextIsCancelled(t *testing.T) {
	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	cfg.Folder = t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	done := make(chan error, 1)
	go func() { done <- serve(cmd, cfg) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve with a cancelled context: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after its context was cancelled")
	}
}
