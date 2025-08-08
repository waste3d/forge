package orchestrator

import (
	"archive/tar" // Добавлен для архивации
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath" // Добавлен для работы с путями
	"sync"
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
	networkName := fmt.Sprintf("forge-network-%s", o.appName)
	o.sendLog("forged-daemon", fmt.Sprintf("Создание сети %s...", networkName))

	networkResp, err := o.dockerClient.NetworkCreate(ctx, networkName, types.NetworkCreate{})
	if err != nil {
		return fmt.Errorf("не удалось создать сеть %s: %w", networkName, err)
	}
	networkID := networkResp.ID
	if err := o.stateManager.AddResource(o.appName, "network", networkID); err != nil {
		o.dockerClient.NetworkRemove(ctx, networkID)
		return fmt.Errorf("критическая ошибка: не удалось сохранить состояние для сети %s: %w", networkID, err)
	}
	o.sendLog("forged-daemon", fmt.Sprintf("Сеть %s создана.", networkName))

	var wg sync.WaitGroup

	for _, dbConfig := range config.Databases {
		wg.Add(1)
		go func(dbConfig parser.DBConfig) {
			defer wg.Done()
			if err := o.startDatabase(ctx, &dbConfig, networkID); err != nil {
				o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка запуска базы данных: %v", err))
			}
		}(dbConfig)
	}

	for _, serviceConfig := range config.Services {
		wg.Add(1)
		go func(serviceConfig parser.ServiceConfig) {
			defer wg.Done()
			if err := o.startService(ctx, &serviceConfig, networkID); err != nil {
				o.sendLog(serviceConfig.Name, fmt.Sprintf("Ошибка запуска сервиса: %v", err))
			}
		}(serviceConfig)
	}

	wg.Wait()
	return nil
}

func (o *Orchestrator) Down(ctx context.Context, appName string) error {
	// ... (без изменений)
	log.Printf("Начинаю процедуру Down для приложения '%s'...", appName)
	resources, err := o.stateManager.GetResourceByApp(appName)
	if err != nil {
		return fmt.Errorf("не удалось получить ресурсы: %w", err)
	}
	if len(resources) == 0 {
		log.Printf("Нет ресурсов для приложения '%s'", appName)
		return nil
	}
	// ... остальной код Down без изменений ...
	return nil
}

func (o *Orchestrator) startDatabase(ctx context.Context, dbConfig *parser.DBConfig, networkID string) error {
	// ... (без изменений)
	o.sendLog(dbConfig.Name, "Starting database...")

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
