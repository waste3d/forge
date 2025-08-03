package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

type Manager struct {
	db *sql.DB
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
		created_at datetime default current_timestamp,
	)
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
