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
	pb "github.com/waste3d/forge/proto"
)

type Orchestrator struct {
	dockerClient *client.Client
	appName      string
	stream       pb.Forge_UpServer
}

func New(appName string, stream pb.Forge_UpServer) (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания клиента Docker: %v", err)
	}

	return &Orchestrator{
		dockerClient: cli,
		appName:      appName,
		stream:       stream,
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

	containerName := fmt.Sprintf("%s-%s", dbConfig.Name, uuid.New().String())

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
