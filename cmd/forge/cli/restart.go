// cmd/forge/cli/restart.go
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var forceRestart bool

var restartCmd = &cobra.Command{
	Use:   "restart [appName]",
	Short: "Перезапускает окружение приложения",
	Args:  cobra.MaximumNArgs(1),
	Run:   runRestart,
}

func init() {
	restartCmd.Flags().BoolVarP(&forceRestart, "force", "f", false, "Игнорировать ошибки при остановке и пытаться запустить снова")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) {
	var appName string
	if len(args) > 0 {
		appName = args[0]
	}

	if err := runDownLogic(appName); err != nil {
		if forceRestart {
			errorLog(os.Stderr, "\n⚠️ Ошибка при остановке: %v (игнорируем из-за --force)\n", err)
		} else {
			errorLog(os.Stderr, "\n❌ Ошибка при остановке: %v\n", err)
			os.Exit(1)
		}
	}

	if err := runUpLogic(); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка при запуске: %v\n", err)
		os.Exit(1)
	}

	successLog("\n✅ Команда 'restart' успешно завершена.\n")
}
