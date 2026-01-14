package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information - set via ldflags during build
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("migrate %s\n", Version)
		if verbose {
			fmt.Printf("  commit: %s\n", Commit)
			fmt.Printf("  built:  %s\n", BuildDate)
		}
	},
}
