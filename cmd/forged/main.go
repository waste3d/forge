package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/waste3d/forge/internal/parser"
	pb "github.com/waste3d/forge/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type forgeServer struct {
	pb.UnimplementedForgeServer
}

func (s *forgeServer) Up(req *pb.UpRequest, stream pb.Forge_UpServer) error {
	log.Println("Получен Up-запрос...")

	config, err := parser.Parse([]byte(req.GetConfigContent()))
	if err != nil {
		log.Printf("Ошибка парсинга конфигурации: %v", err)
		return status.Errorf(codes.InvalidArgument, "ошибка парсинга forge.yaml: %v", err)
	}

	// --- НОВЫЙ БЛОК: Строгая валидация конфигурации ---
	log.Println("Проверка конфигурации...")

	// 1. Проверяем версию.
	if config.Version != 1 {
		errText := fmt.Sprintf("неподдерживаемая версия конфигурации: %d. Поддерживается только версия 1", config.Version)
		log.Println(errText)
		return status.Errorf(codes.InvalidArgument, errText)
	}
	log.Println(" -> Версия [OK]")

	// 2. Проверяем, что appName указано в конфиге. Это теперь обязательное поле.
	if config.AppName == "" {
		errText := "в файле forge.yaml не указано обязательное поле 'appName'"
		log.Println(errText)
		return status.Errorf(codes.InvalidArgument, errText)
	}
	log.Printf(" -> Имя приложения: %s [OK]", config.AppName)
	// --- КОНЕЦ НОВОГО БЛОКА ---

	// Теперь мы используем appName только из конфига
	appNameFromConfig := config.AppName
	log.Printf("Начинаем работу над приложением: %s", appNameFromConfig)

	stream.Send(&pb.LogEntry{
		ServiceName: "forged-daemon",
		Message:     fmt.Sprintf("Конфигурация для '%s' принята и проверена.", appNameFromConfig),
	})
	time.Sleep(500 * time.Millisecond)
	stream.Send(&pb.LogEntry{
		ServiceName: "forged-daemon",
		Message:     "Начинаю подготовку к запуску сервисов...",
	})

	log.Printf("Обработка запроса для '%s' завершена.", appNameFromConfig)
	return nil
}

func main() {
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatalf("Не удалось запустить listener: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterForgeServer(s, &forgeServer{})
	log.Println("Демон 'forged' запущен на порту :9001...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Ошибка gRPC сервера: %v", err)
	}
}
