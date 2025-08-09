package orchestrator

import (
	"archive/tar" // Добавлен для архивации
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath" // Добавлен для работы с путями
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/waste3d/forge/internal/parser"
	"github.com/waste3d/forge/internal/state"
	pb "github.com/waste3d/forge/proto"
	"golang.org/x/sync/errgroup"
)

type Orchestrator struct {
	dockerClient *client.Client
	appName      string
	stream       pb.Forge_UpServer
	stateManager *state.Manager
	logger       *slog.Logger
}

func New(appName string, stream pb.Forge_UpServer, logger *slog.Logger) (*Orchestrator, error) {
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
	if err := o.stateManager.AddResource(o.appName, "network", networkID); err != nil {
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

func (o *Orchestrator) startDatabase(ctx context.Context, dbConfig *parser.DBConfig, networkID string) error {
	o.sendLog(dbConfig.Name, "Starting database...")
	o.logger.Info("запуск базы данных", "dbName", dbConfig.Name, "type", dbConfig.Type, "version", dbConfig.Version)

	if dbConfig.Type == "" || dbConfig.Version == "" {
		return fmt.Errorf("у базы данных '%s' должны быть указаны 'type' и 'version'", dbConfig.Name)
	}
	imageName := fmt.Sprintf("%s:%s", dbConfig.Type, dbConfig.Version)

	reader, err := o.dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка при извлечении образа: %v", err))
		return err
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	containerName := fmt.Sprintf("%s-%s-db", dbConfig.Name, uuid.New().String())

	containerConfig := &container.Config{
		Image: imageName,
		Env:   dbConfig.Env,
	}

	portMap := nat.PortMap{}
	if dbConfig.Port > 0 {
		internalPort := dbConfig.InternalPort
		if internalPort == 0 && dbConfig.Type == "postgres" {
			internalPort = 5432
		}
		if internalPort > 0 {
			portMap[nat.Port(fmt.Sprintf("%d/tcp", internalPort))] = []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", dbConfig.Port)},
			}
		}
	}
	hostConfig := &container.HostConfig{
		PortBindings: portMap,
	}

	resp, err := o.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkID: {Aliases: []string{dbConfig.Name}},
		},
	}, nil, containerName)
	if err != nil {
		return err
	}

	if err := o.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	return o.stateManager.AddResource(o.appName, "container", resp.ID)
}

// startService - ГЛАВНОЕ ИЗМЕНЕНИЕ
func (o *Orchestrator) startService(ctx context.Context, serviceConfig *parser.ServiceConfig, networkID string) error {
	o.sendLog(serviceConfig.Name, "Начинаю запуск сервиса...")

	// --- 1. Определяем контекст сборки (Build Context) ---
	var buildContextPath string
	if serviceConfig.Repo != "" {
		tempDir, err := os.MkdirTemp("", "forge-build-*")
		if err != nil {
			return fmt.Errorf("не удалось создать временную директорию: %w", err)
		}
		defer os.RemoveAll(tempDir)

		o.sendLog(serviceConfig.Name, fmt.Sprintf("Клонирование репозитория %s...", serviceConfig.Repo))
		cmd := exec.Command("git", "clone", "--depth=1", serviceConfig.Repo, tempDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("не удалось склонировать репозиторий: %w, вывод: %s", err, string(output))
		}
		buildContextPath = tempDir
	} else if serviceConfig.Path != "" {
		o.sendLog(serviceConfig.Name, fmt.Sprintf("Используется локальный путь: %s", serviceConfig.Path))
		if _, err := os.Stat(serviceConfig.Path); os.IsNotExist(err) {
			return fmt.Errorf("локальный путь '%s' не найден", serviceConfig.Path)
		}
		buildContextPath = serviceConfig.Path
	} else {
		return fmt.Errorf("у сервиса '%s' должен быть указан либо 'repo', либо 'path'", serviceConfig.Name)
	}

	// --- 2. Собираем Docker образ, передавая контекст как TAR-архив ---
	imageTag := fmt.Sprintf("forge-image-%s-%s:latest", o.appName, serviceConfig.Name)
	o.sendLog(serviceConfig.Name, fmt.Sprintf("Подготовка и сборка Docker-образа из '%s'...", buildContextPath))

	// Создаем Pipe для потоковой передачи данных из архиватора в Docker SDK
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	// Запускаем архивацию в отдельной горутине, чтобы не блокировать основной поток
	go func() {
		// Важно закрывать Writer'ы, чтобы Pipe получил сигнал EOF (конец файла)
		defer pw.Close()
		defer tw.Close()

		// Рекурсивно обходим директорию buildContextPath
		filepath.Walk(buildContextPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Пропускаем директории
			if !info.Mode().IsRegular() {
				return nil
			}

			// Получаем относительный путь файла внутри контекста
			relPath, err := filepath.Rel(buildContextPath, path)
			if err != nil {
				return fmt.Errorf("не удалось получить относительный путь для %s: %w", path, err)
			}

			// Создаем заголовок для TAR-архива
			hdr, err := tar.FileInfoHeader(info, relPath)
			if err != nil {
				return fmt.Errorf("не удалось создать заголовок tar для %s: %w", path, err)
			}
			hdr.Name = relPath // Убеждаемся, что имя в архиве правильное

			if err := tw.WriteHeader(hdr); err != nil {
				return fmt.Errorf("не удалось записать заголовок tar: %w", err)
			}

			// Открываем и копируем содержимое файла в архив
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("не удалось открыть файл %s: %w", path, err)
			}
			defer file.Close()
			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("не удалось скопировать содержимое файла %s в архив: %w", path, err)
			}

			return nil
		})
	}()

	// Передаем архив (pr) в качестве контекста сборки
	buildResp, err := o.dockerClient.ImageBuild(ctx, pr, types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageTag},
		Remove:     true, // Удалять промежуточные контейнеры
		// Поле Context теперь не используется, так как контекст передается как io.Reader
	})
	if err != nil {
		return fmt.Errorf("ошибка при запуске сборки образа: %w", err)
	}
	defer buildResp.Body.Close()

	// Стримим логи сборки клиенту
	scanner := bufio.NewScanner(buildResp.Body)
	for scanner.Scan() {
		o.sendLog(serviceConfig.Name, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		o.sendLog(serviceConfig.Name, fmt.Sprintf("Ошибка чтения логов сборки: %v", err))
	}
	o.sendLog(serviceConfig.Name, "Образ успешно собран.")

	// --- 3. Запускаем контейнер (этот блок без изменений) ---
	containerName := fmt.Sprintf("%s-%s-service", serviceConfig.Name, uuid.New().String())
	portMap := nat.PortMap{}
	if serviceConfig.Port > 0 {
		portMap[nat.Port(fmt.Sprintf("%d/tcp", serviceConfig.Port))] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", serviceConfig.Port)},
		}
	}

	resp, err := o.dockerClient.ContainerCreate(ctx,
		&container.Config{Image: imageTag},
		&container.HostConfig{PortBindings: portMap},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkID: {Aliases: []string{serviceConfig.Name}},
			},
		}, nil, containerName)

	if err != nil {
		return fmt.Errorf("ошибка создания контейнера: %w", err)
	}

	if err := o.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("ошибка запуска контейнера: %w", err)
	}

	o.sendLog(serviceConfig.Name, fmt.Sprintf("Контейнер %s запущен. ID: %s", containerName, resp.ID[:12]))

	return o.stateManager.AddResource(o.appName, "container", resp.ID)
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
