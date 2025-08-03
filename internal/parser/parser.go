package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func Parse(content []byte) (*Config, error) {
	var config Config

	err := yaml.Unmarshal(content, &config)
	if err != nil {
		return nil, fmt.Errorf("ошибка при парсинге конфига: %v", err)
	}

	return &config, nil
}
