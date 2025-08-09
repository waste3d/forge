package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
)

const (
	daemonAddress = "localhost:9001"
	version       = "v0.1.0"
)

var (
	infoLog    = color.New(color.FgYellow).Printf
	successLog = color.New(color.FgGreen).Printf
	errorLog   = color.New(color.FgRed).Fprintf
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "Forge - оркестратор сред разработки",
}

func Execute() error {
	return rootCmd.Execute()
}

func PrintLogs(stream pb.Forge_UpClient) error {
	cDaemon := color.New(color.FgCyan)
	cDB := color.New(color.FgGreen)
	cService := color.New(color.FgMagenta) // Новый цвет для сервисов

	for {
		logEntry, err := stream.Recv()
		if err == io.EOF {
			return nil // Нормальное завершение стрима
		}
		if err != nil {
			return fmt.Errorf("критическая ошибка при чтении потока от сервера: %w", err)
		}

		serviceName := logEntry.GetServiceName()
		message := logEntry.GetMessage()

		// Выбираем цвет в зависимости от имени сервиса
		switch serviceName {
		case "forged-daemon":
			cDaemon.Printf("[%s] %s\n", serviceName, message)
		case "main-db": // Можно оставить специфичный цвет для БД
			cDB.Printf("[%s] %s\n", serviceName, message)
		default:
			// Все остальные сервисы будут одного цвета
			cService.Printf("[%s] %s\n", serviceName, message)
		}
	}
}

func isDaemonRunning() bool {
	conn, err := net.DialTimeout("tcp", daemonAddress, 1*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// startDaemon ищет forged в PATH и запускает его в фоне.
func startDaemon() error {
	path, err := exec.LookPath("forged")
	if err != nil {
		return errors.New("не удалось найти 'forged' в вашем PATH. Убедитесь, что демон установлен и доступен")
	}

	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить 'forged': %w", err)
	}
	infoLog("Демон 'forged' запущен с PID: %d. Он будет работать в фоне.\n", cmd.Process.Pid)
	return nil
}
