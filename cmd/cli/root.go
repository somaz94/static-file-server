// Package cli provides the Cobra command-line interface.
package cli

import (
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/somaz94/static-file-server/internal/config"
	"github.com/somaz94/static-file-server/internal/server"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "static-file-server",
	Short: "Lightweight static file server",
	Long:  "A lightweight, zero-dependency static file server with directory listing, CORS, TLS, and access control.",
	RunE:  runServe,
	// Silence Cobra's default usage/error output to keep logs clean.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "",
		"path to YAML config file")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// runServe loads configuration and starts the server.
func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	return serve(cmd, cfg)
}

// serve runs the server until SIGINT or SIGTERM.
func serve(cmd *cobra.Command, cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return server.Run(ctx, cfg)
}
