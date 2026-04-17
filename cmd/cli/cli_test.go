package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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
