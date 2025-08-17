package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Собирает образы для сервисов из forge.yaml",
	Long:  "Читает forge.yaml и инициирует сборку Docker-образов для указанных сервисов без запуска окружения. Если сервисы не указаны, собирает все.",
	Run:   runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) {
	servicesToBuild := args

	if err := runBuildLogic(servicesToBuild); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'build': %v\n", err)
		os.Exit(1)
	}

	successLog("\n✅ Команда 'build' успешно завершена.\n")
}

func runBuildLogic(servicesToBuild []string) error {
	if !isDaemonRunning() {
		return fmt.Errorf("демон 'forged' не запущен. Запустите его с помощью 'forge system start'")
	}

	infoLog("Чтение и обработка файла forge.yaml...\n")
	configPath := "forge.yaml"
	yamlContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл forge.yaml: %w", err)
	}

	// Код для преобразования относительных путей в абсолютные, как в `up.go`
	configDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return fmt.Errorf("не удалось определить директорию конфига: %w", err)
	}

	var configData map[string]interface{}
	if err := yaml.Unmarshal(yamlContent, &configData); err != nil {
		return fmt.Errorf("не удалось распарсить YAML для модификации путей: %w", err)
	}

	if services, ok := configData["services"].([]interface{}); ok {
		for _, s := range services {
			if service, ok := s.(map[string]interface{}); ok {
				if path, ok := service["path"].(string); ok && path != "" && !filepath.IsAbs(path) {
					absPath := filepath.Join(configDir, path)
					service["path"] = absPath
				}
			}
		}
	}

	modifiedYamlContent, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("не удалось собрать модифицированный YAML: %w", err)
	}
	// --- Конец блока с путями ---

	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к демону: %w", err)
	}
	defer conn.Close()
	client := pb.NewForgeClient(conn)

	req := &pb.BuildRequest{
		ConfigContent: string(modifiedYamlContent),
		ServicesName:  servicesToBuild,
	}

	infoLog("Отправляем Build-запрос демону...\n")
	stream, err := client.Build(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ошибка при вызове Build: %w", err)
	}

	infoLog("Ожидание логов сборки от демона...\n")
	// Переиспользуем существующую функцию для вывода логов
	return PrintLogs(stream)
}
