package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/waste3d/forge/internal/parser"
	"github.com/waste3d/forge/internal/state"
	pb "github.com/waste3d/forge/proto"
)

type Orchestrator struct {
	dockerClient *client.Client
	appName      string
	stream       pb.Forge_UpServer
	stateManager *state.Manager
}

func New(appName string, stream pb.Forge_UpServer) (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента Docker: %v", err)
	}

	sm, err := state.NewManager()
	if err != nil {
		return nil, fmt.Errorf("ошибка создания менеджера состояния: %v", err)
	}

	return &Orchestrator{
		dockerClient: cli,
		appName:      appName,
		stream:       stream,
		stateManager: sm,
	}, nil
}

func (o *Orchestrator) Up(ctx context.Context, config *parser.Config) error {
	var wg sync.WaitGroup

	for _, dbConfig := range config.Databases {
		wg.Add(1)

		go func(dbConfig parser.DBConfig) {
			defer wg.Done()

			if err := o.startDatabase(ctx, &dbConfig); err != nil {
				o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка запуска базы данных: %v", err))
			}
		}(dbConfig)
	}

	wg.Wait()
	return nil
}

func (o *Orchestrator) Down(ctx context.Context, appName string) error {
	log.Printf("Начинаю процедуру Down для приложения '%s'...", appName)

	resources, err := o.stateManager.GetResourceByApp(appName)
	if err != nil {
		return fmt.Errorf("не удалось получить ресурсы: %w", err)
	}

	if len(resources) == 0 {
		log.Printf("Нет ресурсов для приложения '%s'", appName)
		return nil
	}

	var wg sync.WaitGroup
	for _, resource := range resources {
		wg.Add(1)
		go func(res state.Resource) {
			defer wg.Done()
			log.Printf("Удаление ресурса %s (тип: %s)...", res.ID[:12], res.ResourceType)
			timeout := 30

			switch res.ResourceType {
			case "container":
				if err := o.dockerClient.ContainerStop(ctx, res.ID, container.StopOptions{Timeout: &timeout}); err != nil {
					o.sendLog(res.AppName, fmt.Sprintf("Ошибка остановки контейнера %s: %v", res.ID[:12], err))
				}

				if err := o.dockerClient.ContainerRemove(ctx, res.ID, container.RemoveOptions{Force: true}); err != nil {
					log.Printf("ОШИБКА: не удалось удалить контейнер %s: %v", res.ID[:12], err)
					return

				}
				log.Printf("Контейнер %s успешно удален.", res.ID[:12])
			default:
				log.Printf("Неизвестный тип ресурса '%s', пропускаю.", res.ResourceType)
				return
			}

			if err := o.stateManager.RemoveResource(res.ID); err != nil {
				o.sendLog(res.AppName, fmt.Sprintf("Ошибка удаления ресурса %s из состояния: %v", res.ID[:12], err))
			}

		}(resource)
	}
	wg.Wait()
	log.Printf("Процедура Down для приложения '%s' завершена", appName)
	return nil
}

func (o *Orchestrator) startDatabase(ctx context.Context, dbConfig *parser.DBConfig) error {
	o.sendLog(dbConfig.Name, "Starting database...")

	imageName := fmt.Sprintf("%s:%s", dbConfig.Type, dbConfig.Version)

	o.sendLog(dbConfig.Name, fmt.Sprintf("Pulling image %s...", imageName))

	reader, err := o.dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка при извлечении образа: %v", err))
		return err
	}
	io.Copy(os.Stdout, reader)
	o.sendLog(dbConfig.Name, "Image pulled successfully")

	containerName := fmt.Sprintf("%s-%s-db", dbConfig.Name, uuid.New().String())

	containerConfig := &container.Config{
		Image: imageName,
		Env: []string{
			"POSTGRES_PASSWORD=secretpassword", // TODO: Добавить пароль из конфига
		},
	}

	hostPort := fmt.Sprintf("%d", dbConfig.Port)

	containerPort := nat.Port(fmt.Sprintf("%s/tcp", "5432")) // "5432/tcp" - стандартный порт Postgres
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			containerPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			},
		},
	}

	o.sendLog(dbConfig.Name, fmt.Sprintf("Starting container %s...", containerName))
	resp, err := o.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка создания контейнера: %v", err))
		return err
	}

	o.sendLog(dbConfig.Name, fmt.Sprintf("Container %s created with ID: %s", containerName, resp.ID))

	if err := o.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка запуска контейнера: %v", err))
		return err
	}

	containerID := resp.ID

	err = o.stateManager.AddResource(o.appName, "container", containerID)
	if err != nil {
		errMsg := fmt.Sprintf("КРИТИЧЕСКАЯ ОШИБКА: не удалось сохранить состояние для контейнера %s: %v", containerID[:12], err)
		o.sendLog(dbConfig.Name, errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	o.sendLog(dbConfig.Name, fmt.Sprintf("Состояние для контейнера %s сохранено.", containerID[:12]))

	o.sendLog(dbConfig.Name, fmt.Sprintf("База данных успешно запущена на порту localhost:%d", dbConfig.Port))
	return nil
}

func (o *Orchestrator) sendLog(serviceName, message string) {
	entry := &pb.LogEntry{
		ServiceName: serviceName,
		Timestamp:   time.Now().Unix(),
		Message:     message,
	}
	if err := o.stream.Send(entry); err != nil {
		log.Printf("Не удалось отправить лог клиенту: %v", err)
	}
}
