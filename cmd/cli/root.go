// Package cli provides the Cobra command-line interface.
package cli

import (
	"fmt"
	"os"

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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	return server.Run(cfg)
}
