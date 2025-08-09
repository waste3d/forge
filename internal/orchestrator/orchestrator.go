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
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"github.com/waste3d/forge/internal/state"
	"github.com/waste3d/forge/pkg/parser"
	"golang.org/x/sync/errgroup"
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

	o.sendLog("forged-daemon", fmt.Sprintf("Сеть %s создана.", networkName))
	o.logger.Info("сеть успешно создана", "networkName", networkName, "networkID", networkID)

	g, gCtx := errgroup.WithContext(ctx)

	for _, dbConfig := range config.Databases {
		dbConfig := dbConfig
		g.Go(func() error {
			err := o.startDatabase(gCtx, &dbConfig, networkID)
			if err != nil {
				o.logger.Error("ошибка запуска базы данных", "dbName", dbConfig.Name, "error", err)
				o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка запуска: %v", err))
			}
			return err
		})
	}

	for _, serviceConfig := range config.Services {
		serviceConfig := serviceConfig

		g.Go(func() error {
			err := o.startService(gCtx, &serviceConfig, networkID)
			if err != nil {
				o.logger.Error("ошибка запуска сервиса", "serviceName", serviceConfig.Name, "error", err)
				o.sendLog(serviceConfig.Name, fmt.Sprintf("Ошибка запуска: %v", err))
			}
			return err
		})
	}

	o.logger.Info("ожидание завершения всех сервисов и баз данных...")
	return g.Wait()
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

func (o *Orchestrator) healthCheckPort(ctx context.Context, serviceName string, port int) error {
	if port == 0 {
		o.sendLog(serviceName, "Проверка готовности пропущена: порт не указан.")
		return nil
	}

	o.sendLog(serviceName, fmt.Sprintf("Проверка готовности на порту %d...", port))

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := fmt.Sprintf("%s:%d", serviceName, port)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("сервис '%s' не стал доступен по таймауту", serviceName)
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 1*time.Second)
			if err == nil {
				conn.Close()
				o.sendLog(serviceName, "Сервис считается готовым.")
				return nil
			}
			o.sendLog(serviceName, fmt.Sprintf("Сервис запущен, ожидаем стабилизации... (%v)", err))
			time.Sleep(5 * time.Second)
		}

	}

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
