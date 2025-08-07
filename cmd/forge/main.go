package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/waste3d/forge/proto"
)

const daemonAddress = "localhost:9001"

// Глобальные цветные логгеры для удобства
var (
	infoLog    = color.New(color.FgYellow).PrintfFunc()
	successLog = color.New(color.FgGreen).PrintfFunc()
	errorLog   = color.New(color.FgRed).FprintFunc()
)

// --- Главная команда ---
var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "Forge - оркестратор сред разработки",
}

// --- Команда Boot ---
var bootCmd = &cobra.Command{
	Use:   "boot",
	Short: "Запускает демон (если нужно) и поднимает окружение",
	Run: func(cmd *cobra.Command, args []string) {
		// Вызываем основную логику. Если она вернет ошибку...
		if err := runBootLogic(); err != nil {
			// ...печатаем ее красным в стандартный поток ошибок (stderr)
			errorLog(os.Stderr, "\n❌ Ошибка выполнения 'boot': %v\n", err)
			os.Exit(1)
		}
		// Если ошибок не было, печатаем сообщение об успехе
		successLog("\n✅ Команда 'boot' успешно завершена.")
	},
}

// runBootLogic содержит всю логику для команды boot и возвращает ошибку при неудаче.
func runBootLogic() error {
	// 1. Проверяем, запущен ли демон
	if isDaemonRunning() {
		infoLog("Демон 'forged' уже запущен.")
	} else {
		infoLog("Демон 'forged' не найден. Запускаем его в фоновом режиме...")
		if err := startDaemon(); err != nil {
			return fmt.Errorf("критическая ошибка запуска демона: %w", err)
		}
		// Даем демону время на старт перед тем, как к нему подключаться
		time.Sleep(2 * time.Second)
	}

	// 2. Выполняем логику, аналогичную 'up'
	infoLog("Чтение файла forge.yaml...")
	yamlContent, err := os.ReadFile("forge.yaml")
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл forge.yaml: %w", err)
	}

	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к демону: %w", err)
	}
	defer conn.Close()
	client := pb.NewForgeClient(conn)

	req := &pb.UpRequest{ConfigContent: string(yamlContent)}

	infoLog("Отправляем Up-запрос демону...")
	stream, err := client.Up(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ошибка при вызове Up: %w", err)
	}

	infoLog("Ожидание логов от демона...")
	// Возвращаем ошибку, если она произойдет во время чтения логов
	return printLogs(stream)
}

// --- Команда Down ---
var downCmd = &cobra.Command{
	Use:   "down [appName]",
	Short: "Останавливает и удаляет окружение разработки",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		appName := args[0]
		if err := runDownLogic(appName); err != nil {
			errorLog(os.Stderr, "\n❌ Ошибка выполнения 'down': %v\n", err)
			os.Exit(1)
		}
		// Сообщение об успехе теперь печатается внутри runDownLogic
	},
}

// runDownLogic содержит всю логику для команды down.
func runDownLogic(appName string) error {
	infoLog("Отправка запроса на удаление окружения '%s'...", appName)

	if !isDaemonRunning() {
		return errors.New("демон 'forged' не запущен. Невозможно выполнить команду 'down'. Запустите окружение с помощью 'forge boot'")
	}

	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к демону: %w", err)
	}
	defer conn.Close()
	client := pb.NewForgeClient(conn)

	req := &pb.DownRequest{AppName: appName}
	resp, err := client.Down(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ошибка при вызове Down: %w", err)
	}

	successLog("Получен ответ от демона: %s", resp.GetMessage())
	successLog("\n✅ Команда 'down' успешно завершена.")
	return nil
}

// --- Вспомогательные функции ---

// isDaemonRunning проверяет, слушает ли кто-то порт демона.
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
	infoLog("Демон 'forged' запущен с PID: %d. Он будет работать в фоне.", cmd.Process.Pid)
	return nil
}

// printLogs обрабатывает и выводит цветные логи из gRPC стрима. Теперь возвращает ошибку.
func printLogs(stream pb.Forge_UpClient) error {
	cDaemon := color.New(color.FgCyan)
	cDB := color.New(color.FgGreen)
	cDefault := color.New(color.FgWhite)

	for {
		logEntry, err := stream.Recv()
		if err == io.EOF {
			// Это не ошибка, а нормальное завершение стрима.
			return nil
		}
		if err != nil {
			// А вот это уже реальная ошибка связи с сервером.
			return fmt.Errorf("критическая ошибка при чтении потока от сервера: %w", err)
		}

		serviceName := logEntry.GetServiceName()
		message := logEntry.GetMessage()

		switch serviceName {
		case "forged-daemon":
			cDaemon.Printf("[%s] %s\n", serviceName, message)
		case "main-db":
			cDB.Printf("[%s] %s\n", serviceName, message)
		default:
			cDefault.Printf("[%s] %s\n", serviceName, message)
		}
	}
}

func main() {
	// Убираем стандартные префиксы даты/времени из логов,
	// так как мы полностью контролируем их вывод через fatih/color.
	log.SetFlags(0)

	rootCmd.AddCommand(bootCmd)
	rootCmd.AddCommand(downCmd)

	// Выполняем корневую команду. Ошибки парсинга флагов
	// или неизвестных команд будут обработаны здесь самой библиотекой Cobra.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
