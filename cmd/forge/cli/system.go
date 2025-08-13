package cli

import (
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Управление демоном 'forged'",
}

var systemStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Запускает демон 'forged' в фоновом режиме",
	Run: func(cmd *cobra.Command, args []string) {
		if isDaemonRunning() {
			infoLog("Демон 'forged' уже запущен.\n")
			return
		}
		infoLog("Запускаем демон 'forged'...\n")
		if err := startDaemon(); err != nil {
			errorLog(os.Stderr, "❌ Не удалось запустить демон: %v\n", err)
			os.Exit(1)
		}

		const maxRetries = 5
		const retryDelay = 1 * time.Second

		var ready bool
		for i := 0; i < maxRetries; i++ {
			if isDaemonRunning() {
				ready = true
				break
			}
			time.Sleep(retryDelay)
		}

		if !ready {
			errorLog(os.Stderr, "❌ Демон был запущен, но не стал доступен в течение %d секунд.\n", maxRetries)
			errorLog(os.Stderr, "Проверьте логи демона: ~/.forge/forged.log\n")
			os.Exit(1)
		}
		// --- КОНЕЦ ИСПРАВЛЕНИЯ ---

		successLog("✅ Демон успешно запущен.\n")
	},
}

// systemStopCmd - для остановки демона
var systemStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Останавливает демон 'forged'",
	Run: func(cmd *cobra.Command, args []string) {
		if !isDaemonRunning() {
			infoLog("Демон 'forged' не запущен.\n")
			return
		}
		infoLog("Останавливаем демон 'forged'...\n")
		// Самый простой и надежный способ для начала - pkill
		cmdKill := exec.Command("pkill", "forged")
		if err := cmdKill.Run(); err != nil {
			errorLog(os.Stderr, "❌ Не удалось остановить демон (возможно, pkill не установлен или демон уже остановлен): %v\n", err)
			os.Exit(1)
		}
		successLog("✅ Демон успешно остановлен.\n")
	},
}

// systemStatusCmd - для проверки статуса демона
var systemStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Проверяет статус демона 'forged'",
	Run: func(cmd *cobra.Command, args []string) {
		if isDaemonRunning() {
			successLog("✅ Демон 'forged' запущен и работает.\n")
		} else {
			infoLog("ℹ️ Демон 'forged' не запущен.\n")
		}
	},
}

func init() {
	// Добавляем дочерние команды к 'system'
	systemCmd.AddCommand(systemStartCmd)
	systemCmd.AddCommand(systemStopCmd)
	systemCmd.AddCommand(systemStatusCmd)

	// Добавляем команду 'system' к корневой команде
	rootCmd.AddCommand(systemCmd)
}
