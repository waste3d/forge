package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var psCmd = &cobra.Command{
	Use:   "ps [appName]",
	Short: "Показывает статус запущенных сервисов",
	Long:  "Показывает статус запущенных сервисов. Если [appName] не указан, показывает сервисы для всех приложений.",
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
		if appName != "" {
			infoLog("Для приложения '%s' не найдено запущенных ресурсов.\n", appName)
		} else {
			infoLog("Не найдено запущенных ресурсов.\n")
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "APP NAME\tNAME\tSTATUS\tAGE\tPORTS")

	for _, s := range resp.GetServices() {
		age := "N/A"
		createdTime, err := time.Parse(time.RFC3339, s.Created)
		if err == nil {
			age = units.HumanDuration(time.Since(createdTime))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.GetAppName(), s.GetServiceName(), s.GetStatus(), age, s.GetPorts())
	}

	return w.Flush()
}
