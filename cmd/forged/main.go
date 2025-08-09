package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
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
	logger *slog.Logger
}

func (s *forgeServer) Up(req *pb.UpRequest, stream pb.Forge_UpServer) error {
	s.logger.Info("получен Up-запрос")

	config, err := parser.Parse([]byte(req.GetConfigContent()))
	if err != nil {
		s.logger.Error("ошибка парсинга forge.yaml", "error", err)
		return status.Errorf(codes.InvalidArgument, "ошибка парсинга forge.yaml: %v", err)
	}

	if config.Version != 1 {
		err := fmt.Errorf("неподдерживаемая версия конфигурации: %d", config.Version)
		s.logger.Error("неверная версия конфига", "version", config.Version)
		return status.Error(codes.InvalidArgument, err.Error())
	}

	if config.AppName == "" {
		err := errors.New("в файле forge.yaml не указано обязательное поле 'appName'")
		s.logger.Error(err.Error())
		return status.Error(codes.InvalidArgument, err.Error())
	}

	appName := config.AppName
	s.logger.Info("конфигурация проверена", "appName", appName)

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

	orch, err := orchestrator.New(appName, stream, s.logger)
	if err != nil {
		s.logger.Error("критическая ошибка инициализации оркестратора", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации: %v", err)
	}

	err = orch.Up(context.Background(), config)
	if err != nil {
		s.logger.Error("ошибка выполнения оркестрации", "appName", appName, "error", err)
		return status.Errorf(codes.Internal, "ошибка выполнения оркестрации: %v", err)
	}

	s.logger.Info("оркестрация успешно завершена", "appName", appName)
	return nil
}

func (s *forgeServer) Down(ctx context.Context, req *pb.DownRequest) (*pb.DownResponse, error) {
	appName := req.GetAppName()
	s.logger.Info("получен Down-запрос", "appName", appName)

	if appName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "в запросе не указано обязательное поле 'appName'")
	}

	orch, err := orchestrator.New(appName, nil, s.logger)
	if err != nil {
		s.logger.Error("критическая ошибка инициализации оркестратора", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка инициализации оркестратора: %v", err)
	}

	err = orch.Down(ctx, appName)
	if err != nil {
		s.logger.Error("ошибка выполнения Down", "appName", appName, "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка выполнения оркестрации: %v", err)
	}

	s.logger.Info("процедура Down успешно завершена", "appName", appName)
	return &pb.DownResponse{
		Message: fmt.Sprintf("Процедура Down для приложения '%s' успешно завершена", appName),
	}, nil
}

func main() {

	handler := slog.NewJSONHandler(os.Stderr, nil)
	logger := slog.New(handler)

	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		slog.Error("не удалось запустить listener", "error", err)
	}

	s := grpc.NewServer()
	pb.RegisterForgeServer(s, &forgeServer{logger: logger})
	slog.Info("дemon 'forged' запущен на порту :9001")
	if err := s.Serve(lis); err != nil {
		slog.Error("ошибка gRPC сервера", "error", err)
	}
}
