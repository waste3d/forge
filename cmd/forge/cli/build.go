package cli

import "github.com/spf13/cobra"

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Собирает образы для сервисов из forge.yaml",
	Long:  "Читает forge.yaml и инициирует сборку Docker-образов для указанных сервисов без запуска окружения.",
	Run:   runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) {

}
