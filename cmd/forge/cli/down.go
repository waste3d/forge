package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/waste3d/forge/cmd/forge/cli/helpers"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Останавливает демон и все сервисы",
	Args:  cobra.MaximumNArgs(1),
	Run:   runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) {
	var appName string
	var err error

	if len(args) > 0 {
		appName = args[0]
	} else {
		appName, err = helpers.GetAppNameFromConfig()
		if err != nil {
			errorLog(os.Stderr, "\n❌ %v\n", err)
			os.Exit(1)
		}
	}

	if err := runDownLogic(appName); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'down': %v\n", err)
		os.Exit(1)
	}

	successLog("\n✅ Команда 'down' успешно завершена.\n")
}

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
	return nil
}
