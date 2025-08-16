package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/waste3d/forge/cmd/forge/cli/helpers"
)

var restartCmd = &cobra.Command{
	Use:   "restart [appName] <serviceName>",
	Short: "Перезапускает окружение приложения",
	Args:  cobra.RangeArgs(0, 1),
	Run:   runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) {
	var appName string

	if len(args) > 0 {
		appName = args[0]
	} else {
		var err error
		appName, err = helpers.GetAppNameFromConfig()
		if err != nil {
			errorLog(os.Stderr, "\n❌ %v\n", err)
			os.Exit(1)
		}
	}

	if err := runDownLogic(appName); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'restart': %v\n", err)
		os.Exit(1)
	}

	if err := runUpLogic(); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'restart': %v\n", err)
		os.Exit(1)
	}

	successLog("\n✅ Команда 'restart' успешно завершена.\n")
}
