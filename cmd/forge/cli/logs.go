package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var logsCmd = &cobra.Command{
	Use:   "logs [appName] [serviceName]",
	Short: "Показывает логи для одного или всех сервисов приложения",
	Long:  "Показывает логи для указанного appName. Если serviceName не указан, показывает логи для всех сервисов.",
	Args:  cobra.RangeArgs(1, 2),
	Run:   runLogs,
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Следить за логами в реальном времени")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) {
	appName := args[0]
	serviceName := ""

	if len(args) > 1 {
		serviceName = args[1]
	}
	follow, _ := cmd.Flags().GetBool("follow")

	if err := runLogsLogic(cmd.Context(), appName, serviceName, follow); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'logs': %v\n", err)
		os.Exit(1)
	}
	successLog("\n✅ Команда 'logs' успешно завершена.\n")
}

func runLogsLogic(ctx context.Context, appName, serviceName string, follow bool) error {
	if !isDaemonRunning() {
		return errors.New("демон 'forged' не запущен. Невозможно получить логи")
	}

	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к демону: %w", err)
	}
	defer conn.Close()

	client := pb.NewForgeClient(conn)

	req := &pb.LogRequest{
		AppName:     appName,
		ServiceName: serviceName,
		Follow:      follow,
	}

	infoLog("Получаем логи для %s/%s...\n", appName, serviceName)
	stream, err := client.Logs(ctx, req)
	if err != nil {
		return fmt.Errorf("ошибка при получении логов: %w", err)
	}

	return PrintLogs(stream)
}
