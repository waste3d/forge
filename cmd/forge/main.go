package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "Forge - это простой оркестратор сред разработки",
	Long: `Forge помогает разработчикам быстро разворачивать и управлять
	сложными проектами локально с помощью одного простого YAML файла.`,
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Создает и запускает среду разработки согласно forge.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Подключение к демону 'forged' на localhost:9001...")

		conn, err := grpc.Dial("localhost:9001", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("Не удалось подключиться к демону: %v. Убедитесь, что демон 'forged' запущен.", err)
		}
		defer conn.Close()

		log.Println("Соединение установлено.")

		client := pb.NewForgeClient(conn)

		req := &pb.UpRequest{
			AppName:       "my-first-app",
			ConfigContent: "services:\n  - name: web\n  - name: db",
		}

		log.Println("Отправка Up-запроса демону...")

		stream, err := client.Up(context.Background(), req)
		if err != nil {
			log.Fatalf("Ошибка при отправке запроса: %v", err)
		}

		log.Println("Ожидание логов от демона...")
		for {
			logEntry, err := stream.Recv()
			if err == io.EOF {
				log.Println("Сервер завершил передачу логов.")
				break
			}
			if err != nil {
				log.Fatalf("Критическая ошибка при чтении потока от сервера: %v", err)
			}
			fmt.Printf("[%s]: %s\n", logEntry.GetServiceName(), logEntry.GetMessage())
		}
		fmt.Println("\nКоманда 'up' успешно завершена.")

	},
}

func main() {
	rootCmd.AddCommand(upCmd)
	// TODO: Здесь мы будем добавлять другие команды, например, `downCmd`.

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
