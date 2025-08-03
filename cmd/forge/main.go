package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	// "path/filepath" // Больше не нужен

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
		// --- УДАЛЕНО: Блок определения appName по пути ---

		log.Println("Чтение файла forge.yaml...")
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

		// --- ИЗМЕНЕНИЕ: Поле AppName в запросе больше не используется ---
		// Демон сам извлечет имя из конфига. Мы можем передать пустую строку,
		// хотя в будущем это поле можно будет использовать для переопределения
		// имени через флаг командной строки, например: `forge up --name my-override`
		req := &pb.UpRequest{
			AppName:       "", // Демон должен извлечь имя из ConfigContent
			ConfigContent: string(yamlContent),
		}

		log.Println("Отправляем Up-запрос демону...")
		stream, err := client.Up(context.Background(), req)
		if err != nil {
			// Теперь здесь могут быть осмысленные ошибки от сервера!
			log.Fatalf("Ошибка при вызове Up: %v", err)
		}

		// ... остальная часть функции Run остается без изменений
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
			fmt.Printf("[%s] %s\n", logEntry.GetServiceName(), logEntry.GetMessage())
		}
		fmt.Println("\nКоманда 'up' успешно завершена.")
	},
}

func main() {
	rootCmd.AddCommand(upCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
