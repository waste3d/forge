package cli

import (
	"os"
	"os/exec"

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
