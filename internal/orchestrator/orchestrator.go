package orchestrator

import (
	// Добавлен для архивации

	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog" // Добавлен для работы с путями
	"net"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-units"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"github.com/waste3d/forge/internal/state"
	"github.com/waste3d/forge/pkg/parser"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Orchestrator struct {
	dockerClient *client.Client
	appName      string
	stream       pb.Forge_UpServer
	stateManager *state.Manager
	logger       *slog.Logger
}

func New(appName string, stream pb.Forge_UpServer, logger *slog.Logger, sm *state.Manager) (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента Docker: %v", err)
	}

	return &Orchestrator{
		dockerClient: cli,
		appName:      appName,
		stream:       stream,
		stateManager: sm,
		logger:       logger.With("appName", appName),
	}, nil
}

func (o *Orchestrator) Up(ctx context.Context, config *parser.Config) error {
	networkName := fmt.Sprintf("forge-network-%s", o.appName)
	o.sendLog("forged-daemon", fmt.Sprintf("Создание сети %s...", networkName))

	networkResp, err := o.dockerClient.NetworkCreate(ctx, networkName, types.NetworkCreate{})
	if err != nil {
		o.logger.Error("не удалось создать сеть %s", "error", err)
		return fmt.Errorf("не удалось создать сеть %s: %w", networkName, err)
	}

	networkID := networkResp.ID
	if err := o.stateManager.AddResource(o.appName, "network", networkID, "forged-daemon"); err != nil {
		o.dockerClient.NetworkRemove(ctx, networkID)
		return fmt.Errorf("критическая ошибка: не удалось сохранить состояние для сети %s: %w", networkID, err)
	}

	var allNodes []Node
	for i := range config.Databases {
		allNodes = append(allNodes, &DBNode{&config.Databases[i], config.Databases[i].Port})
	}

	for i := range config.Services {
		allNodes = append(allNodes, &ServiceNode{&config.Services[i], config.Services[i].Port})
	}

	sortedNodes, err := Sort(allNodes)
	if err != nil {
		o.logger.Error("не удалось отсортировать узлы", "error", err)
		return fmt.Errorf("не удалось отсортировать узлы: %w", err)
	}

	for _, node := range sortedNodes {
		nodeName := node.GetName()
		o.sendLog("forged-daemon", fmt.Sprintf("Запуск %s...", nodeName))

		if err := node.Start(ctx, networkID, o); err != nil {
			o.logger.Error("ошибка запуска узла", "nodeName", nodeName, "error", err)
			o.sendLog(nodeName, fmt.Sprintf("Ошибка запуска: %v", err))
			return fmt.Errorf("ошибка запуска узла %s: %w", nodeName, err)
		}

		if err := node.IsReady(ctx, o); err != nil {
			o.logger.Error("ошибка проверки готовности узла", "nodeName", nodeName, "error", err)
			o.sendLog(nodeName, fmt.Sprintf("Ошибка проверки готовности: %v", err))
			return fmt.Errorf("ошибка проверки готовности узла %s: %w", nodeName, err)
		}

		o.logger.Info("узел успешно запущен и готов", "nodeName", nodeName)

		o.sendLog(nodeName, "Узел успешно запущен и готов.")
	}

	o.sendLog("forged-daemon", fmt.Sprintf("Сеть %s создана.", networkName))
	o.logger.Info("сеть успешно создана", "networkName", networkName, "networkID", networkID)

	return nil
}

// Down останавливает и удаляет все ресурсы, связанные с приложением.
func (o *Orchestrator) Down(ctx context.Context, appName string) error {
	o.logger.Info("начинаю процедуру Down", "appName", appName)

	resources, err := o.stateManager.GetResourceByApp(appName)
	if err != nil {
		o.logger.Error("не удалось получить ресурсы из state manager", "error", err)
		return fmt.Errorf("не удалось получить ресурсы: %w", err)
	}

	if len(resources) == 0 {
		o.logger.Warn("не найдено ресурсов для приложения. процедура Down завершена.", "appName", appName)
		return nil
	}

	g, _ := errgroup.WithContext(ctx)
	var networkIDs []string

	for _, res := range resources {
		res := res
		switch res.ResourceType {
		case "container":
			g.Go(func() error {
				o.logger.Info("остановка и удаление контейнера", "containerID", res.ID)

				timeout := 30
				if err := o.dockerClient.ContainerStop(context.Background(), res.ID, container.StopOptions{Timeout: &timeout}); err != nil {
					if !client.IsErrNotFound(err) {
						o.logger.Error("не удалось остановить контейнер", "containerID", res.ID, "error", err)
						return err
					}
					o.logger.Warn("контейнер не найден, возможно, был удален ранее", "containerID", res.ID)
				}

				if err := o.dockerClient.ContainerRemove(context.Background(), res.ID, container.RemoveOptions{Force: true}); err != nil {
					if !client.IsErrNotFound(err) {
						o.logger.Error("не удалось удалить контейнер", "containerID", res.ID, "error", err)
						return err
					}
				}

				if err := o.stateManager.RemoveResource(res.ID); err != nil {
					o.logger.Error("не удалось удалить ресурс из состояния", "resourceID", res.ID, "error", err)
					return err
				}

				o.logger.Info("контейнер успешно удален", "containerID", res.ID)
				return nil
			})

		case "network":
			networkIDs = append(networkIDs, res.ID)
		}
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("произошла ошибка при удалении контейнеров: %w", err)
	}

	for _, netID := range networkIDs {
		o.logger.Info("удаление сети", "networkID", netID)
		if err := o.dockerClient.NetworkRemove(context.Background(), netID); err != nil {
			if !client.IsErrNotFound(err) {
				o.logger.Error("не удалось удалить сеть", "networkID", netID, "error", err)
			}
		}

		if err := o.stateManager.RemoveResource(netID); err != nil {
			o.logger.Error("не удалось удалить ресурс сети из состояния", "resourceID", netID, "error", err)
		}
		o.logger.Info("сеть успешно удалена", "networkID", netID)
	}

	o.logger.Info("процедура Down успешно завершена", "appName", appName)
	return nil
}

func (o *Orchestrator) Logs(ctx context.Context, serviceName string, follow bool, stream pb.Forge_LogsServer) error {
	resources, err := o.stateManager.GetResourceByApp(o.appName)
	if err != nil {
		o.logger.Error("не удалось получить ресурсы из state manager", "error", err)
		return fmt.Errorf("не удалось получить ресурсы: %w", err)
	}

	g, gCtx := errgroup.WithContext(ctx)

	var matchFound bool

	for _, res := range resources {
		if res.ResourceType != "container" {
			continue
		}

		if serviceName != "" && res.ServiceName != serviceName {
			continue
		}

		matchFound = true

		res := res

		g.Go(func() error {
			logOptions := container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     follow,
				Timestamps: false,
				Tail:       "100",
			}

			logReader, err := o.dockerClient.ContainerLogs(gCtx, res.ID, logOptions)
			if err != nil {
				o.logger.Error("не удалось получить логи контейнера", "containerID", res.ID, "error", err)
				return err
			}
			defer logReader.Close()

			r, w := io.Pipe()

			go func() {
				defer w.Close()
				_, err := stdcopy.StdCopy(w, w, logReader)
				if err != nil {
					o.logger.Error("ошибка при демультиплексировании логов", "error", err)
				}
			}()

			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				logEntry := &pb.LogEntry{
					ServiceName: res.ServiceName,
					Message:     scanner.Text(),
				}

				if err := stream.Send(logEntry); err != nil {
					o.logger.Error("не удалось отправить лог клиенту", "error", err)
					return err
				}
			}

			return scanner.Err()
		})
	}
	if serviceName != "" && !matchFound {
		o.logger.Warn("сервис не найден", "serviceName", serviceName)
		stream.Send(&pb.LogEntry{
			ServiceName: "forged-daemon",
			Message:     fmt.Sprintf("Ошибка: сервис с именем '%s' не найден в приложении '%s'.", serviceName, o.appName),
		})
	}

	return g.Wait()
}

func (o *Orchestrator) Exec(stream pb.Forge_ExecServer) error {
	initPlayload, err := stream.Recv()
	if err != nil {
		o.logger.Error("не удалось прочитать начальные параметры exec", "error", err)
		return status.Errorf(codes.InvalidArgument, "не удалось прочитать начальные параметры: %v", err)
	}

	setup := initPlayload.GetSetup()
	if setup == nil {
		return status.Errorf(codes.InvalidArgument, "первое сообщение от клиента должно содержать 'ExecSetup'")
	}

	appName := setup.GetAppName()
	serviceName := setup.GetServiceName()
	command := setup.GetCommand()

	o.logger.Info("получен exec-запрос", "app", appName, "service", serviceName, "command", command)

	resources, err := o.stateManager.GetResourceByApp(appName)
	if err != nil {
		return status.Errorf(codes.Internal, "ошибка получения ресурсов из state manager: %v", err)
	}

	var containerID string
	for _, res := range resources {
		if res.ResourceType == "container" && res.ServiceName == serviceName {
			containerID = res.ID
			break
		}
	}

	if containerID == "" {
		return status.Errorf(codes.NotFound, "сервис '%s' в приложении '%s' не найден или не запущен", serviceName, appName)
	}

	ctx := stream.Context()
	execConfig := &types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          setup.GetTty(),
		Cmd:          command,
	}

	execResp, err := o.dockerClient.ContainerExecCreate(ctx, containerID, *execConfig)
	if err != nil {
		return status.Errorf(codes.Internal, "не удалось создать exec-инстанс в Docker: %v", err)
	}

	hijackedResp, err := o.dockerClient.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: setup.GetTty()})
	if err != nil {
		return status.Errorf(codes.Internal, "не удалось присоединиться к exec-инстансу: %v", err)
	}
	defer hijackedResp.Close()

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return hijackedResp.CloseWrite()
			}
			if err != nil {
				return err
			}

			_, err = hijackedResp.Conn.Write(req.GetStdin())
			if err != nil {
				return err
			}
		}
	})

	g.Go(func() error {
		for {
			buf := make([]byte, 4096)
			n, err := hijackedResp.Conn.Read(buf)
			if err != nil {
				return err
			}

			if n > 0 {
				if err := stream.Send(&pb.ExecOutput{Data: buf[:n]}); err != nil {
					return err
				}
			}
		}
	})

	return g.Wait()
}

func (o *Orchestrator) Build(ctx context.Context, config *parser.Config, servicesToBuild []string) error {
	o.sendLog("forged-daemon", "Начинаю процедуру сборки образов...")

	servicesMap := make(map[string]bool)
	for _, name := range servicesToBuild {
		servicesMap[name] = true
	}

	buildAll := len(servicesToBuild) == 0

	for i := range config.Services {
		service := &config.Services[i]

		if !buildAll && !servicesMap[service.Name] {
			continue
		}

		if service.Image != "" {
			o.sendLog(service.Name, fmt.Sprintf("Пропуск сборки: используется готовый образ '%s'.", service.Image))
			continue
		}

		if service.Path == "" && service.Repo == "" {
			o.sendLog(service.Name, "Пропуск сборки: не указан путь или репозиторий.")
			continue
		}

		if _, err := o.buildService(ctx, service); err != nil {
			o.sendLog(service.Name, fmt.Sprintf("Ошибка сборки: %v", err))
			return err
		}
	}

	o.sendLog("forged-daemon", "Сборка образов завершена.")
	return nil
}

func (o *Orchestrator) healthCheckPort(ctx context.Context, serviceName string, port int, timeout int) error {
	if port == 0 {
		o.sendLog(serviceName, "Проверка готовности пропущена: порт не указан.")
		return nil
	}

	o.sendLog(serviceName, fmt.Sprintf("Проверка готовности на порту %d...", port))

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	address := fmt.Sprintf("localhost:%d", port)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("сервис '%s' не стал доступен по таймауту", serviceName)
		case <-ticker.C:
			// Пытаемся установить соединение с проброшенным портом
			conn, err := net.DialTimeout("tcp", address, 1*time.Second)
			if err == nil {
				// Успех!
				conn.Close()
				o.sendLog(serviceName, fmt.Sprintf("Сервис готов и отвечает на порту %d.", port))
				return nil
			}
			// Логируем попытку для отладки, но не спамим в основной лог
			o.logger.Debug("попытка health check не удалась", "service", serviceName, "addr", address, "error", err)
		}
	}

}

func (o *Orchestrator) Status(ctx context.Context, appName string) ([]*pb.ServiceStatus, error) {
	o.logger.Info("получение статуса для приложения", "appName", appName)

	var resources []state.Resource
	var err error

	if appName == "" {
		resources, err = o.stateManager.GetAllResources()
		if err != nil {
			return nil, fmt.Errorf("не удалось получить ресурсы из state manager: %w", err)
		}
	} else {
		resources, err = o.stateManager.GetResourceByApp(appName)
	}

	if len(resources) == 0 {
		return []*pb.ServiceStatus{}, nil
	}

	var statuses []*pb.ServiceStatus

	for _, res := range resources {
		if res.ResourceType != "container" {
			continue
		}

		inspect, err := o.dockerClient.ContainerInspect(ctx, res.ID)
		if err != nil {
			if client.IsErrNotFound(err) {
				statuses = append(statuses, &pb.ServiceStatus{
					AppName:      res.AppName,
					ServiceName:  res.ServiceName,
					ResourceType: res.ResourceType,
					ResourceId:   "not found",
					Status:       "Stale (removed outside of Forge)",
				})
				continue
			}
			return nil, fmt.Errorf("не удалось инспектировать контейнер %s: %w", res.ID, err)
		}

		var portMappings []string
		if inspect.HostConfig != nil {
			for port, bindings := range inspect.HostConfig.PortBindings {
				if len(bindings) > 0 {
					for _, binding := range bindings {
						mapping := fmt.Sprintf("%s:%s->%s", binding.HostIP, binding.HostPort, port.Port())
						portMappings = append(portMappings, mapping)
					}
				}
			}
		}

		var statusString string
		if inspect.State.Running {
			startTime, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
			if err == nil {
				duration := units.HumanDuration(time.Since(startTime))
				statusString = fmt.Sprintf("Up %s", duration)
			} else {
				statusString = "Running"
			}
		} else {
			statusString = fmt.Sprintf("Exited (%d)", inspect.State.ExitCode)
		}

		status := &pb.ServiceStatus{
			AppName:      res.AppName,
			ServiceName:  res.ServiceName,
			ResourceType: res.ResourceType,
			ResourceId:   inspect.ID[:12],
			Created:      inspect.Created,
			Status:       statusString,
			Ports:        strings.Join(portMappings, ", "),
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (o *Orchestrator) sendLog(serviceName, message string) {
	entry := &pb.LogEntry{
		ServiceName: serviceName,
		Timestamp:   time.Now().Unix(),
		Message:     message,
	}
	if o.stream != nil {
		if err := o.stream.Send(entry); err != nil {
			log.Printf("Не удалось отправить лог клиенту: %v", err)
		}
	}
}
