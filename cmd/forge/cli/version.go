package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Выводит версию приложения",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("forge version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
