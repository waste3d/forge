package helpers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/waste3d/forge/pkg/parser"
	"gopkg.in/yaml.v2"
)

func GetAppNameFromConfig() (string, error) {
	content, err := os.ReadFile("forge.yaml")
	if err != nil {
		return "", fmt.Errorf("ошибка чтения forge.yaml: %w. Пожалуйста, укажите appName явно или запустите команду из директории с файлом forge.yaml", err)
	}

	config, err := parser.Parse(content)
	if err != nil {
		return "", fmt.Errorf("ошибка парсинга forge.yaml: %w", err)
	}

	if config.AppName == "" {
		return "", fmt.Errorf("appName не указан в forge.yaml. Пожалуйста, укажите appName явно или запустите команду из директории с файлом forge.yaml")
	}

	return config.AppName, nil
}

func LoadAndPrepareConfig(configPath string) ([]byte, error) {
	yamlContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
	}

	configDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return nil, fmt.Errorf("не удалось определить директорию конфига: %w", err)
	}

	var configData map[string]interface{}
	if err := yaml.Unmarshal(yamlContent, &configData); err != nil {
		return nil, fmt.Errorf("не удалось распарсить YAML для модификации путей: %w", err)
	}

	if services, ok := configData["services"].([]interface{}); ok {
		for _, s := range services {
			if service, ok := s.(map[string]interface{}); ok {
				if path, ok := service["path"].(string); ok && path != "" && !filepath.IsAbs(path) {
					absPath := filepath.Join(configDir, path)
					service["path"] = absPath
				}
			}
		}
	}

	modifiedYamlContent, err := yaml.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("не удалось собрать модифицированный YAML: %w", err)
	}

	return modifiedYamlContent, nil
}
