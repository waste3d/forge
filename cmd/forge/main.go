package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/fatih/color" // НОВЫЙ ИМПОРТ
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/waste3d/forge/proto"
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "Forge - оркестратор сред разработки",
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Создает и запускает среду разработки согласно forge.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		// ИЗМЕНЕНИЕ: Используем цветные логгеры для самого клиента
		log.SetOutput(os.Stdout)
		log.SetFlags(0)
		infoLog := color.New(color.FgYellow).PrintlnFunc()

		infoLog("Чтение файла forge.yaml...")
		yamlContent, err := os.ReadFile("forge.yaml")
		if err != nil {
			log.Fatalf("Не удалось прочитать файл forge.yaml: %v", err)
		}

		conn, err := grpc.Dial("localhost:9001", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("Не удалось подключиться к демону: %v", err)
		}
		defer conn.Close()
		client := pb.NewForgeClient(conn)

		req := &pb.UpRequest{
			AppName:       "",
			ConfigContent: string(yamlContent),
		}

		infoLog("Отправляем Up-запрос демону...")
		stream, err := client.Up(context.Background(), req)
		if err != nil {
			log.Fatalf("Ошибка при вызове Up: %v", err)
		}

		infoLog("Ожидание логов от демона...")

		// НОВЫЙ БЛОК: Создаем цветные принтеры для разных сервисов
		cDaemon := color.New(color.FgCyan)
		cDB := color.New(color.FgGreen)
		cDefault := color.New(color.FgWhite)

		for {
			logEntry, err := stream.Recv()
			if err == io.EOF {
				color.Green("\nСервер завершил передачу логов.")
				break
			}
			if err != nil {
				log.Fatalf("Критическая ошибка при чтении потока от сервера: %v", err)
			}

			// ИЗМЕНЕНИЕ: Выбираем цвет в зависимости от имени сервиса
			serviceName := logEntry.GetServiceName()
			message := logEntry.GetMessage()

			switch serviceName {
			case "forged-daemon":
				cDaemon.Printf("[%s] %s\n", serviceName, message)
			case "main-db":
				cDB.Printf("[%s] %s\n", serviceName, message)
			// Сюда можно добавлять 'case' для других сервисов, например, 'cache-redis'
			default:
				cDefault.Printf("[%s] %s\n", serviceName, message)
			}
		}
		color.Green("\nКоманда 'up' успешно завершена.")
	},
}

var downCmd = &cobra.Command{
	Use:   "down [appName]",
	Short: "Останавливает и удаляет окружение разработки",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// ИЗМЕНЕНИЕ: Тоже используем цветной вывод
		log.SetOutput(os.Stdout)
		log.SetFlags(0)

		appName := args[0]
		color.Yellow("Отправка запроса на удаление окружения '%s'...", appName)

		conn, err := grpc.Dial("localhost:9001", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("Не удалось подключиться к демону: %v", err)
		}
		defer conn.Close()
		client := pb.NewForgeClient(conn)

		req := &pb.DownRequest{
			AppName: appName,
		}

		resp, err := client.Down(context.Background(), req)
		if err != nil {
			log.Fatalf("Ошибка при вызове Down: %v", err)
		}

		color.Green("Получен ответ от демона: %s", resp.GetMessage())
		color.Green("\nКоманда 'down' успешно завершена.")
	},
}

func main() {
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
