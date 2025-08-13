package helpers

import (
	"fmt"
	"os"

	"github.com/waste3d/forge/pkg/parser"
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
