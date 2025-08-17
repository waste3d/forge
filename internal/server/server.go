package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/docker/docker/client"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"github.com/waste3d/forge/internal/orchestrator"
	"github.com/waste3d/forge/internal/state"
	"github.com/waste3d/forge/pkg/parser"
	"golang.org/x/sync/errgroup"
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

	sm, err := state.NewManager()
	if err != nil {
		s.logger.Error("критическая ошибка инициализации state manager", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации state manager: %v", err)
	}
	defer sm.Close()

	existingResources, err := sm.GetResourceByApp(appName)
	if err != nil {
		s.logger.Error("ошибка проверки существующих ресурсов", "appName", appName, "error", err)
		return status.Errorf(codes.Internal, "ошибка проверки состояния: %v", err)
	}

	if len(existingResources) > 0 {
		errMsg := fmt.Sprintf("окружение для '%s' уже запущено. Пожалуйста, сначала выполните 'forge down %s'", appName, appName)
		s.logger.Warn(errMsg)
		return status.Errorf(codes.AlreadyExists, "%s", errMsg)
	}

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

	orch, err := orchestrator.New(appName, stream, s.logger, sm)
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

	sm, err := state.NewManager()
	if err != nil {
		s.logger.Error("критическая ошибка инициализации state manager", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка инициализации state manager: %v", err)
	}
	defer sm.Close()

	orch, err := orchestrator.New(appName, nil, s.logger, sm)
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

// Logs реализует получение логов для одного или всех сервисов приложения
func (s *forgeServer) Logs(req *pb.LogRequest, stream pb.Forge_LogsServer) error {
	appName := req.GetAppName()
	serviceName := req.GetServiceName()
	follow := req.GetFollow()

	sm, err := state.NewManager()
	if err != nil {
		s.logger.Error("критическая ошибка инициализации state manager", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации state manager: %v", err)
	}
	defer sm.Close()

	s.logger.Info("получен Logs-запрос", "appName", appName, "serviceName", serviceName, "follow", follow)

	orch, err := orchestrator.New(appName, stream, s.logger, sm)
	if err != nil {
		s.logger.Error("критическая ошибка инициализации оркестратора", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации оркестратора: %v", err)
	}

	return orch.Logs(stream.Context(), serviceName, follow, stream)
}

func InitializeServer(listenAddr string) error {
	handler := slog.NewJSONHandler(os.Stderr, nil)
	logger := slog.New(handler)

	sm, err := state.NewManager()
	if err != nil {
		return fmt.Errorf("критическая ошибка инициализации state manager: %w", err)
	}
	defer sm.Close()

	dockerCli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("ошибка создания клиента Docker: %w", err)
	}
	defer dockerCli.Close()

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		lis, err := net.Listen("tcp", listenAddr)
		if err != nil {
			logger.Error("не удалось запустить gRPC listener", "addr", listenAddr, "error", err)
			return fmt.Errorf("не удалось слушать порт gRPC %s: %w", listenAddr, err)
		}

		s := grpc.NewServer()
		pb.RegisterForgeServer(s, &forgeServer{logger: logger})
		logger.Info("gRPC сервер запущен", "addr", listenAddr)

		go func() {
			<-ctx.Done()
			logger.Info("Остановка gRPC сервера...")
			s.GracefulStop()
		}()

		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("ошибка gRPC сервера: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("ошибка в группе запуска сервисов: %w", err)
	}
	return nil
}

func (s *forgeServer) Build(req *pb.BuildRequest, stream pb.Forge_BuildServer) error {
	s.logger.Info("получен Build-запрос")

	config, err := parser.Parse([]byte(req.GetConfigContent()))
	if err != nil {
		s.logger.Error("ошибка парсинга forge.yaml", "error", err)
		return status.Errorf(codes.InvalidArgument, "ошибка парсинга forge.yaml: %v", err)
	}

	appName := config.AppName
	sm, err := state.NewManager()

	if err != nil {
		s.logger.Error("критическая ошибка инициализации state manager", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации state manager: %v", err)
	}
	defer sm.Close()

	orch, err := orchestrator.New(appName, stream, s.logger, sm)
	if err != nil {
		s.logger.Error("критическая ошибка инициализации оркестратора", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации оркестратора: %v", err)
	}

	err = orch.Build(stream.Context(), config, req.GetServicesName())
	if err != nil {
		s.logger.Error("ошибка выполнения сборки", "appName", appName, "error", err)
		return status.Errorf(codes.Internal, "ошибка выполнения сборки: %v", err)
	}

	s.logger.Info("сборка образов завершена", "appName", appName)
	return nil
}

func (s *forgeServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	appName := req.GetAppName()
	s.logger.Info("получен Status-запрос", "appName", appName)

	sm, err := state.NewManager()
	if err != nil {
		s.logger.Error("критическая ошибка инициализации state manager", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка инициализации state manager: %v", err)
	}
	defer sm.Close()

	orch, err := orchestrator.New(appName, nil, s.logger, sm)
	if err != nil {
		s.logger.Error("критическая ошибка инициализации оркестратора", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка инициализации оркестратора: %v", err)
	}

	serviceStatuses, err := orch.Status(ctx, appName)
	if err != nil {
		s.logger.Error("ошибка получения статуса сервисов", "appName", appName, "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка получения статуса сервисов: %v", err)
	}

	return &pb.StatusResponse{
		Services: serviceStatuses,
	}, nil
}

func (s *forgeServer) Exec(stream pb.Forge_ExecServer) error {
	sm, err := state.NewManager()
	if err != nil {
		s.logger.Error("критическая ошибка инициализации state manager", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации state manager: %v", err)
	}
	defer sm.Close()

	orch, err := orchestrator.New("", nil, s.logger, sm)
	if err != nil {
		s.logger.Error("критическая ошибка инициализации оркестратора", "error", err)
		return status.Errorf(codes.Internal, "ошибка инициализации оркестратора: %v", err)
	}

	return orch.Exec(stream)
}
