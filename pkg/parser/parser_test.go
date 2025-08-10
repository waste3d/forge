package parser

import (
	"testing"
)

func TestParse(t *testing.T) {
	// Table-driven tests — стандартный паттерн в Go
	testCases := []struct {
		name        string
		yamlContent []byte
		expectErr   bool
		validate    func(*testing.T, *Config)
	}{
		{
			name: "Успешный парсинг корректного конфига",
			yamlContent: []byte(`
version: 1
appName: my-test-app
databases:
  - name: main-db
    type: postgres
    version: "14"
    port: 5432
services:
  - name: backend
    type: go
    repo: github.com/user/backend
    port: 8080
    internalPort: 8000
    dependsOn:
      - main-db
`),
			expectErr: false,
			validate: func(t *testing.T, c *Config) {
				if c.AppName != "my-test-app" {
					t.Errorf("Ожидалось appName 'my-test-app', получено '%s'", c.AppName)
				}
				if len(c.Databases) != 1 {
					t.Errorf("Ожидалась 1 база данных, получено %d", len(c.Databases))
				}
				if len(c.Services) != 1 {
					t.Errorf("Ожидался 1 сервис, получено %d", len(c.Services))
				}
				if c.Services[0].Name != "backend" {
					t.Errorf("Ожидался сервис с именем 'backend', получено '%s'", c.Services[0].Name)
				}
				if c.Services[0].DependsOn[0] != "main-db" {
					t.Errorf("Ожидалась зависимость 'main-db', получено '%s'", c.Services[0].DependsOn[0])
				}
			},
		},
		{
			name: "Ошибка при некорректном YAML",
			yamlContent: []byte(`
version: 1
appName: my-app
  services: - name: broken
`),
			expectErr: true,
			validate:  nil,
		},
		{
			name:        "Ошибка при пустом контенте",
			yamlContent: []byte(""),
			expectErr:   true,
			validate:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := Parse(tc.yamlContent)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
			} else {
				if err != nil {
					t.Errorf("Неожиданная ошибка: %v", err)
				}
				if tc.validate != nil {
					tc.validate(t, config)
				}
			}
		})
	}
}
