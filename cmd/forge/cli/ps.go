package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"github.com/waste3d/forge/pkg/parser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var psCmd = &cobra.Command{
	Use:   "ps [appName]",
	Short: "Показывает статус запущенных сервисов окружения",
	Long:  "Показывает статус запущенных сервисов для указанного appName. Если appName не указан, пытается найти forge.yaml в текущей директории.",
	Args:  cobra.RangeArgs(0, 1),
	Run:   runPs,
}

func init() {
	rootCmd.AddCommand(psCmd)
}

func runPs(cmd *cobra.Command, args []string) {
	var appName string
	if len(args) > 0 {
		appName = args[0]
	} else {
		// Пытаемся получить appName из локального forge.yaml
		content, err := os.ReadFile("forge.yaml")
		if err != nil {
			errorLog(os.Stderr, "\n❌ Ошибка: не указан appName и не найден forge.yaml в текущей директории.\n")
			os.Exit(1)
		}
		config, err := parser.Parse(content)
		if err != nil {
			errorLog(os.Stderr, "\n❌ Ошибка парсинга forge.yaml: %v\n", err)
			os.Exit(1)
		}
		appName = config.AppName
	}

	if appName == "" {
		errorLog(os.Stderr, "\n❌ Не удалось определить имя приложения (appName).\n")
		os.Exit(1)
	}

	if err := runPsLogic(cmd.Context(), appName); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'ps': %v\n", err)
		os.Exit(1)
	}
}

func runPsLogic(ctx context.Context, appName string) error {
	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к демону: %w", err)
	}
	defer conn.Close()

	client := pb.NewForgeClient(conn)
	req := &pb.StatusRequest{AppName: appName}

	resp, err := client.Status(ctx, req)
	if err != nil {
		return fmt.Errorf("ошибка при вызове Status: %w", err)
	}

	if len(resp.GetServices()) == 0 {
		infoLog("Для приложения '%s' не найдено запущенных ресурсов.\n", appName)
		return nil
	}

	// Используем tabwriter для красивого форматирования таблицы
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tID\tSTATUS\tPORTS")

	for _, s := range resp.GetServices() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ServiceName, s.ResourceId, s.Status, s.Ports)
	}

	return w.Flush()
}
