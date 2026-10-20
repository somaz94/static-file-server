package cli

import (
	"github.com/spf13/cobra"

	"github.com/somaz94/static-file-server/internal/config"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Serve only the GitLab Runner cache endpoint",
	Long: "Serve only the S3 cache endpoint (presigned GET/HEAD/PUT) on S3_PORT, with /healthz and\n" +
		"/metrics, and no file server. Reads the same configuration as the root command;\n" +
		"S3_FOLDER, S3_ACCESS_KEY and S3_SECRET_KEY are required.",
	RunE: runCache,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
}

func runCache(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}
	if err := cfg.EnableCacheOnly(); err != nil {
		return err
	}
	return serve(cmd, cfg)
}
