package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/waste3d/forge/cmd/forge/cli/helpers"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	modifiedYamlContent, err := helpers.LoadAndPrepareConfig(configPath)
	if err != nil {
		return err
	}

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
	return PrintLogs(stream)
}
