package parser

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

func Parse(content []byte) (*Config, error) {
	var config Config

	if len(content) == 0 {
		return nil, errors.New("содержимое конфигурации не может быть пустым")
	}

	err := yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, fmt.Errorf("ошибка при парсинге конфига: %v", err)
	}

	return &config, nil
}
