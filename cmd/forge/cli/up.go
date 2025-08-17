package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/waste3d/forge/cmd/forge/cli/helpers"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Создает и запускает окружение из forge.yaml",
	Long:  "Команда 'up' читает forge.yaml, при необходимости запускает демон 'forged' и разворачивает окружение.",
	Run:   runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) {
	if err := runUpLogic(); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'up': %v\n", err)
		os.Exit(1)
	}
	successLog("\n✅ Команда 'up' успешно завершена.\n")
}

func runUpLogic() error {
	if isDaemonRunning() {
		infoLog("Демон 'forged' уже запущен.\n")
	} else {
		infoLog("Демон 'forged' не найден. Запускаем его в фоновом режиме...\n")
		if err := startDaemon(); err != nil {
			return fmt.Errorf("критическая ошибка запуска демона: %w", err)
		}
		time.Sleep(2 * time.Second)
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

	req := &pb.UpRequest{ConfigContent: string(modifiedYamlContent)}

	infoLog("Отправляем Up-запрос демону...\n")
	stream, err := client.Up(context.Background(), req)
	if err != nil {
		return fmt.Errorf("ошибка при вызове Up: %w", err)
	}

	infoLog("Ожидание логов от демона...\n")
	return PrintLogs(stream)
}
