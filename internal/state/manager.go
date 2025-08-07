package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Resource struct {
	ID           string
	AppName      string
	ResourceType string
}

type Manager struct {
	db *sql.DB
}

func (m *Manager) GetResourceByApp(appName string) ([]Resource, error) {
	query := "SELECT resource_id, app_name, resource_type FROM resources WHERE app_name = ?"
	rows, err := m.db.Query(query, appName)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить ресурсы: %w", err)
	}
	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		var r Resource
		if err := rows.Scan(&r.ID, &r.AppName, &r.ResourceType); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки ресурса: %w", err)
		}
		resources = append(resources, r)
	}

	return resources, nil
}

func (m *Manager) RemoveResource(resourceId string) error {
	query := "DELETE FROM resources WHERE resource_id = ?"
	_, err := m.db.Exec(query, resourceId)
	if err != nil {
		return fmt.Errorf("не удалось удалить ресурс: %w", err)
	}
	return nil
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("не удалось определить домашнюю директорию: %w", err)
	}
	dbPath := filepath.Join(home, ".forge")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию для базы данных: %w", err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(dbPath, "forge.db"))
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть базу данных: %w", err)
	}

	m := &Manager{db: db}

	if err := m.createTables(); err != nil {
		return nil, fmt.Errorf("не удалось создать таблицы: %w", err)
	}
	return m, nil
}

func (m *Manager) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS resources (
		id INTEGER primary key autoincrement,
		app_name text not null,
		resource_type text not null, -- "container", "network", etc.
		resource_id text not null,
		created_at datetime default current_timestamp
	);
	`

	_, err := m.db.Exec(query)
	return err
}

func (m *Manager) AddResource(appName, resourceType, resourceId string) error {
	query := `insert into resources (app_name, resource_type, resource_id) values (?, ?, ?)`
	_, err := m.db.Exec(query, appName, resourceType, resourceId)
	return err
}

func (m *Manager) Close() {
	m.db.Close()
}
