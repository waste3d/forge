package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
	ai "github.com/waste3d/forge/openai"
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
	logsCmd.Flags().Bool("ai", false, "Анализировать логи с помощью ИИ для поиска корневой причины ошибок")
	logsCmd.Flags().String("output", "", "Сохранить результат анализа в файл")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) {
	appName := args[0]
	serviceName := ""

	if len(args) > 1 {
		serviceName = args[1]
	}

	follow, _ := cmd.Flags().GetBool("follow")
	ai, _ := cmd.Flags().GetBool("ai")
	output, _ := cmd.Flags().GetString("output")

	if ai && follow {
		errorLog(os.Stderr, "\n❌ Флаги '--ai' и '--follow' нельзя использовать одновременно.\n")
		os.Exit(1)
	}

	if output != "" && !ai {
		errorLog(os.Stderr, "\n❌ Флаг '--output' требует указания '--ai' для сохранения результата.\n")
		os.Exit(1)
	}

	if err := runLogsLogic(cmd.Context(), appName, serviceName, follow, ai, output); err != nil {
		errorLog(os.Stderr, "\n❌ Ошибка выполнения 'logs': %v\n", err)
		os.Exit(1)
	}
	successLog("\n✅ Команда 'logs' успешно завершена.\n")
}

func runLogsLogic(ctx context.Context, appName, serviceName string, follow, useAI bool, output string) error {
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

	if useAI {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Suffix = " Анализирую логи с помощью ИИ... (может занять до 30 секунд)"
		s.Start()

		// Собираем логи вместо их печати
		collectedLogs := make(map[string][]string)
		for {
			logEntry, err := stream.Recv()
			if err == io.EOF {
				break // Все логи получены
			}
			if err != nil {
				s.Stop()
				return fmt.Errorf("ошибка при чтении потока логов: %w", err)
			}
			collectedLogs[logEntry.GetServiceName()] = append(collectedLogs[logEntry.GetServiceName()], logEntry.GetMessage())
		}

		// Вызываем нашу новую функцию
		aiResponse, err := ai.AnalyzeLogsWithAI(ctx, collectedLogs)
		s.Stop() // Останавливаем спиннер
		if err != nil {
			return err
		}

		// Печатаем результат от ИИ
		fmt.Println("\n---")
		renderer, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		out, err := renderer.Render(aiResponse)
		if err != nil {
			fmt.Println(aiResponse) // fallback — печатаем как есть
		} else {
			fmt.Print(out)
		}
		fmt.Println("---")

		if output != "" {
			os.WriteFile(output, []byte(aiResponse), 0644)
			successLog("Результат анализа сохранен в файл %s\n", output)
		}

		return nil

	} else {
		// Старая логика, если --ai не используется
		err := PrintLogs(stream)
		if err == nil {
			successLog("\n✅ Команда 'logs' успешно завершена.\n")
		}
		return err
	}
}
