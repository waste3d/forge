package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/waste3d/forge/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Выводит версию приложения",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("forge version %s\n", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
