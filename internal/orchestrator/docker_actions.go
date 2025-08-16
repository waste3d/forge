// File: internal/orchestrator/docker_actions.go

package orchestrator

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/waste3d/forge/pkg/parser"
)

type DockerBuildResponse struct {
	Stream      string      `json:"stream"`
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

var buildError error

func (o *Orchestrator) startDatabase(ctx context.Context, dbConfig *parser.DBConfig, networkID string) error {
	o.sendLog(dbConfig.Name, "Starting database service...")

	if dbConfig.Type == "" || dbConfig.Version == "" {
		return fmt.Errorf("у базы данных '%s' должны быть указаны 'type' и 'version'", dbConfig.Name)
	}
	imageName := fmt.Sprintf("%s:%s", dbConfig.Type, dbConfig.Version)

	o.sendLog(dbConfig.Name, fmt.Sprintf("Pulling image %s...", imageName))
	reader, err := o.dockerClient.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Ошибка при извлечении образа: %v", err))
		return err
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	containerConfig := &container.Config{
		Image: imageName,
		Env:   dbConfig.Env,
	}

	hostConfig := &container.HostConfig{}

	if dbConfig.Port > 0 && dbConfig.InternalPort > 0 {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Mapping host port %d to container port %d", dbConfig.Port, dbConfig.InternalPort))

		hostConfig.PortBindings = nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", dbConfig.InternalPort)): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", dbConfig.Port),
				},
			},
		}
	} else if dbConfig.Port > 0 {
		o.sendLog(dbConfig.Name, fmt.Sprintf("Warning: 'port' %d is specified, but 'internalPort' is not. Port will not be exposed.", dbConfig.Port))
	}

	containerName := fmt.Sprintf("forge-%s-%s-%s", o.appName, dbConfig.Name, uuid.New().String()[:8])

	resp, err := o.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkID: {Aliases: []string{dbConfig.Name}},
		},
	}, nil, containerName)
	if err != nil {
		return fmt.Errorf("не удалось создать контейнер для %s: %w", dbConfig.Name, err)
	}

	if err := o.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("не удалось запустить контейнер для %s: %w", dbConfig.Name, err)
	}

	return o.stateManager.AddResource(o.appName, "container", resp.ID, dbConfig.Name)
}

func (o *Orchestrator) startService(ctx context.Context, serviceConfig *parser.ServiceConfig, networkID string) error {
	o.sendLog(serviceConfig.Name, "Начинаю запуск сервиса...")

	var imageTag string

	if serviceConfig.Image != "" {
		imageTag = serviceConfig.Image
		o.sendLog(serviceConfig.Name, fmt.Sprintf("Используется готовый образ: %s. Скачиваю...", imageTag))

		reader, err := o.dockerClient.ImagePull(ctx, imageTag, types.ImagePullOptions{})
		if err != nil {
			msg := fmt.Sprintf("Ошибка при скачивании образа '%s': %v", imageTag, err)
			o.sendLog(serviceConfig.Name, msg)
			return fmt.Errorf("%s", msg)
		}
		io.Copy(io.Discard, reader)
		reader.Close()
		o.sendLog(serviceConfig.Name, "Образ успешно скачан.")

	} else {
		imageTag = fmt.Sprintf("forge-image-%s-%s:latest", o.appName, serviceConfig.Name)
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
			return fmt.Errorf("у сервиса '%s' должен быть указан либо 'image', либо 'repo', либо 'path'", serviceConfig.Name)
		}

		o.sendLog(serviceConfig.Name, fmt.Sprintf("Подготовка и сборка Docker-образа из '%s'...", buildContextPath))

		pr, pw := io.Pipe()
		tw := tar.NewWriter(pw)
		go func() {
			defer pw.Close()
			defer tw.Close()

			filepath.Walk(buildContextPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				relPath, err := filepath.Rel(buildContextPath, path)
				if err != nil {
					return fmt.Errorf("не удалось получить относительный путь для %s: %w", path, err)
				}
				hdr, err := tar.FileInfoHeader(info, relPath)
				if err != nil {
					return fmt.Errorf("не удалось создать заголовок tar для %s: %w", path, err)
				}
				hdr.Name = relPath
				if err := tw.WriteHeader(hdr); err != nil {
					return fmt.Errorf("не удалось записать заголовок tar: %w", err)
				}
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

		// Запускаем сборку образа
		buildResp, err := o.dockerClient.ImageBuild(ctx, pr, types.ImageBuildOptions{
			Dockerfile: "Dockerfile",
			Tags:       []string{imageTag},
			Remove:     true, // Удалять промежуточные контейнеры
		})
		if err != nil {
			return fmt.Errorf("ошибка при запуске сборки образа: %w", err)
		}
		defer buildResp.Body.Close()

		// Стримим логи сборки пользователю
		scanner := bufio.NewScanner(buildResp.Body)
		for scanner.Scan() {
			live := scanner.Bytes()
			var respLine DockerBuildResponse
			if err := json.Unmarshal(live, &respLine); err != nil {
				o.sendLog(serviceConfig.Name, scanner.Text())
				continue
			}
			if respLine.Stream != "" {
				o.sendLog(serviceConfig.Name, respLine.Stream)
			}
			if respLine.Error != "" {
				buildError = fmt.Errorf("ошибка сборки образа: %s", respLine.ErrorDetail.Message)
			}
		}
		if err := scanner.Err(); err != nil {
			o.sendLog(serviceConfig.Name, fmt.Sprintf("Ошибка чтения логов сборки: %v", err))
		}
		if buildError != nil {
			o.sendLog(serviceConfig.Name, buildError.Error())
			return buildError
		}
		o.sendLog(serviceConfig.Name, "Образ успешно собран.")
	}

	// --- ОБЩАЯ ЧАСТЬ: ЗАПУСК КОНТЕЙНЕРА ПОСЛЕ ПОЛУЧЕНИЯ ОБРАЗА ---
	o.sendLog(serviceConfig.Name, fmt.Sprintf("Создание и запуск контейнера из образа '%s'...", imageTag))
	containerName := fmt.Sprintf("forge-%s-%s-%s", o.appName, serviceConfig.Name, uuid.New().String()[:8])

	portMap := nat.PortMap{}
	if serviceConfig.Port > 0 && serviceConfig.InternalPort > 0 {
		portMap[nat.Port(fmt.Sprintf("%d/tcp", serviceConfig.InternalPort))] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", serviceConfig.Port)},
		}
	}

	resp, err := o.dockerClient.ContainerCreate(ctx,
		&container.Config{
			Image: imageTag,
			Env:   serviceConfig.Env,
		},
		&container.HostConfig{
			PortBindings: portMap,
		},
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

	// Сохраняем информацию о созданном ресурсе
	return o.stateManager.AddResource(o.appName, "container", resp.ID, serviceConfig.Name)
}

func (o *Orchestrator) buildService(ctx context.Context, serviceConfig *parser.ServiceConfig) (string, error) {
	imageTag := fmt.Sprintf("forge-image-%s-%s:latest", o.appName, serviceConfig.Name)
	var buildContextPath string

	if serviceConfig.Repo != "" {
		tempDir, err := os.MkdirTemp(".", "forge-build-*")
		if err != nil {
			return "", fmt.Errorf("не удалось создать временную директорию: %w", err)
		}
		defer os.RemoveAll(tempDir)

		o.sendLog(serviceConfig.Name, fmt.Sprintf("Клонирование репозитория %s...", serviceConfig.Repo))
		cmd := exec.Command("git", "clone", "--depth=1", serviceConfig.Repo, tempDir)

		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("не удалось склонировать репозиторий: %w, вывод: %s", err, string(output))
		}

		buildContextPath = tempDir
	} else if serviceConfig.Path != "" {
		o.sendLog(serviceConfig.Name, fmt.Sprintf("Используется локальный путь: %s", serviceConfig.Path))

		if _, err := os.Stat(serviceConfig.Path); os.IsNotExist(err) {
			return "", fmt.Errorf("локальный путь '%s' не найден", serviceConfig.Path)
		}

		buildContextPath = serviceConfig.Path
	} else {
		return "", fmt.Errorf("у сервиса '%s' должен быть указан либо 'repo', либо 'path'", serviceConfig.Name)
	}

	o.sendLog(serviceConfig.Name, fmt.Sprintf("Подготовка и сборка Docker-образа из '%s'...", buildContextPath))

	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer tw.Close()

		filepath.Walk(buildContextPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			relPath, err := filepath.Rel(buildContextPath, path)
			if err != nil {
				return fmt.Errorf("не удалось получить относительный путь для %s: %w", path, err)
			}

			hdr, err := tar.FileInfoHeader(info, relPath)
			if err != nil {
				return fmt.Errorf("не удалось создать заголовок tar для %s: %w", path, err)
			}

			hdr.Name = relPath
			if err := tw.WriteHeader(hdr); err != nil {
				return fmt.Errorf("не удалось записать заголовок tar: %w", err)
			}

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

	buildResp, err := o.dockerClient.ImageBuild(ctx, pr, types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageTag},
		Remove:     true,
	})

	if err != nil {
		return "", fmt.Errorf("ошибка при запуске сборки образа: %w", err)
	}
	defer buildResp.Body.Close()

	var buildError error
	scanner := bufio.NewScanner(buildResp.Body)
	for scanner.Scan() {
		live := scanner.Bytes()
		var respLine DockerBuildResponse
		if err := json.Unmarshal(live, &respLine); err != nil {
			o.sendLog(serviceConfig.Name, scanner.Text())
			continue
		}
		if respLine.Stream != "" {
			o.sendLog(serviceConfig.Name, respLine.Stream)
		}
		if respLine.Error != "" {
			buildError = fmt.Errorf("ошибка сборки образа: %s", respLine.ErrorDetail.Message)
		}
	}

	if buildError != nil {
		return "", buildError
	}

	o.sendLog(serviceConfig.Name, "Образ успешно собран.")
	return imageTag, nil
}
