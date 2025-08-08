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
	"path/filepath" // Добавлен для работы с путями
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3" // Добавлен для манипуляции с YAML

	pb "github.com/waste3d/forge/proto"
)

const daemonAddress = "localhost:9001"

// Глобальные цветные логгеры для удобства
var (
	infoLog    = color.New(color.FgYellow).Printf
	successLog = color.New(color.FgGreen).Printf
	errorLog   = color.New(color.FgRed).Fprintf
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
		if err := runBootLogic(); err != nil {
			errorLog(os.Stderr, "\n❌ Ошибка выполнения 'boot': %v\n", err)
			os.Exit(1)
		}
		successLog("\n✅ Команда 'boot' успешно завершена.\n")
	},
}

// runBootLogic содержит всю логику для команды boot.
func runBootLogic() error {
	// 1. Проверяем и запускаем демон (если нужно)
	if isDaemonRunning() {
		infoLog("Демон 'forged' уже запущен.\n")
	} else {
		infoLog("Демон 'forged' не найден. Запускаем его в фоновом режиме...\n")
		if err := startDaemon(); err != nil {
			return fmt.Errorf("критическая ошибка запуска демона: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	// 2. Читаем и обрабатываем forge.yaml
	infoLog("Чтение и обработка файла forge.yaml...\n")
	configPath := "forge.yaml"
	yamlContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл forge.yaml: %w", err)
	}

	// --- НОВЫЙ БЛОК: Преобразование путей в абсолютные ---
	// Определяем абсолютную директорию, где лежит forge.yaml
	configDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return fmt.Errorf("не удалось определить директорию конфига: %w", err)
	}

	// Парсим YAML в общую структуру, чтобы найти и изменить поля 'path'
	var configData map[string]interface{}
	if err := yaml.Unmarshal(yamlContent, &configData); err != nil {
		return fmt.Errorf("не удалось распарсить YAML для модификации путей: %w", err)
	}

	// Ищем секцию 'services' и проходимся по ней
	if services, ok := configData["services"].([]interface{}); ok {
		for _, s := range services {
			if service, ok := s.(map[string]interface{}); ok {
				// Если есть поле 'path' и оно не является абсолютным...
				if path, ok := service["path"].(string); ok && path != "" && !filepath.IsAbs(path) {
					// ...преобразуем его в абсолютный путь, соединив с директорией конфига
					absPath := filepath.Join(configDir, path)
					service["path"] = absPath // Заменяем относительный путь на абсолютный
					infoLog("Преобразован относительный путь '%s' в '%s'\n", path, absPath)
				}
			}
		}
	}

	// Преобразуем модифицированную структуру обратно в YAML-строку
	modifiedYamlContent, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("не удалось собрать модифицированный YAML: %w", err)
	}
	// --- КОНЕЦ НОВОГО БЛОКА ---

	// 3. Подключаемся к демону и отправляем запрос
	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к демону: %w", err)
	}
	defer conn.Close()
	client := pb.NewForgeClient(conn)

	// ОТПРАВЛЯЕМ МОДИФИЦИРОВАННЫЙ КОНТЕНТ
	req := &pb.UpRequest{ConfigContent: string(modifiedYamlContent)}

	infoLog("Отправляем Up-запрос демону...\n")
	stream, err := client.Up(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ошибка при вызове Up: %w", err)
	}

	infoLog("Ожидание логов от демона...\n")
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
	},
}

// runDownLogic содержит всю логику для команды down.
func runDownLogic(appName string) error {
	infoLog("Отправка запроса на удаление окружения '%s'...\n", appName)

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

	successLog("Получен ответ от демона: %s\n", resp.GetMessage())
	successLog("\n✅ Команда 'down' успешно завершена.\n")
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
	infoLog("Демон 'forged' запущен с PID: %d. Он будет работать в фоне.\n", cmd.Process.Pid)
	return nil
}

// printLogs обрабатывает и выводит цветные логи из gRPC стрима. Теперь возвращает ошибку.
func printLogs(stream pb.Forge_UpClient) error {
	cDaemon := color.New(color.FgCyan)
	cDB := color.New(color.FgGreen)
	cService := color.New(color.FgMagenta) // Новый цвет для сервисов
	// cDefault := color.New(color.FgWhite)

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

func main() {
	// Убираем стандартные префиксы даты/времени из логов
	log.SetFlags(0)

	rootCmd.AddCommand(bootCmd)
	rootCmd.AddCommand(downCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
