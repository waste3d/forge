package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/waste3d/forge/internal/orchestrator"
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
		return status.Errorf(codes.InvalidArgument, "ошибка парсинга forge.yaml: %v", err)
	}

	if config.Version != 1 {
		return status.Errorf(codes.InvalidArgument, "неподдерживаемая версия конфигурации: %d", config.Version)
	}
	if config.AppName == "" {
		return status.Errorf(codes.InvalidArgument, "в файле forge.yaml не указано обязательное поле 'appName'")
	}
	appName := config.AppName
	log.Printf("Конфигурация для '%s' проверена.", appName)

	stream.Send(&pb.LogEntry{
		ServiceName: "forged-daemon",
		Timestamp:   time.Now().Unix(),
		Message:     fmt.Sprintf("Конфигурация для '%s' принята и проверена.", appName),
	})
	time.Sleep(200 * time.Millisecond)
	stream.Send(&pb.LogEntry{
		ServiceName: "forged-daemon",
		Timestamp:   time.Now().Unix(),
		Message:     "Начинаю оркестрацию...",
	})

	orch, err := orchestrator.New(appName, stream)
	if err != nil {
		log.Printf("Критическая ошибка инициализации оркестратора: %v", err)
		return status.Errorf(codes.Internal, "ошибка инициализации: %v", err)
	}

	err = orch.Up(context.Background(), config)
	if err != nil {
		log.Printf("Оркестрация для '%s' провалилась: %v", appName, err)
		return status.Errorf(codes.Internal, "ошибка выполнения оркестрации: %v", err)
	}

	log.Printf("Оркестрация для '%s' успешно завершена.", appName)
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
