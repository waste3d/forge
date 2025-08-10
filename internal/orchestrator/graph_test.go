package orchestrator

import (
	"context"
	"reflect"
	"testing"

	"github.com/waste3d/forge/pkg/parser"
)

type mockNode struct {
	name          string
	deps          []string
	isReadyCalled bool
	startCalled   bool
}

func (m *mockNode) GetName() string           { return m.name }
func (m *mockNode) GetDependencies() []string { return m.deps }

func (m *mockNode) Start(ctx context.Context, networkID string, orchestrator *Orchestrator) error {
	m.startCalled = true
	return nil
}
func (m *mockNode) IsReady(ctx context.Context, orchestrator *Orchestrator) error {
	m.isReadyCalled = true
	return nil
}

func TestSort(t *testing.T) {
	testCases := []struct {
		name          string
		nodes         []Node
		expectedOrder []string
		expectErr     bool
	}{
		{
			name: "Простая последовательная зависимость",
			nodes: []Node{
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "backend", DependsOn: []string{"db"}}},
				&DBNode{DBConfig: &parser.DBConfig{Name: "db"}},
			},
			expectedOrder: []string{"db", "backend"},
			expectErr:     false,
		},
		{
			name: "Более сложный граф",
			nodes: []Node{
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "frontend", DependsOn: []string{"backend"}}},
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "backend", DependsOn: []string{"auth-service", "db"}}},
				&DBNode{DBConfig: &parser.DBConfig{Name: "db"}},
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "auth-service", DependsOn: []string{"db"}}},
			},

			expectedOrder: []string{"db", "auth-service", "backend", "frontend"},
			expectErr:     false,
		},
		{
			name: "Обнаружение цикла зависимостей",
			nodes: []Node{
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "service-a", DependsOn: []string{"service-b"}}},
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "service-b", DependsOn: []string{"service-a"}}},
			},
			expectedOrder: nil,
			expectErr:     true,
		},
		{
			name: "Зависимость от несуществующего узла",
			nodes: []Node{
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "service-a", DependsOn: []string{"non-existent"}}},
			},
			expectedOrder: nil,
			expectErr:     true,
		},
		{
			name: "Нет зависимостей",
			nodes: []Node{
				&ServiceNode{ServiceConfig: &parser.ServiceConfig{Name: "service-a"}},
				&DBNode{DBConfig: &parser.DBConfig{Name: "db"}},
			},
			expectedOrder: []string{"service-a", "db"},
			expectErr:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sorted, err := Sort(tc.nodes)

			if tc.expectErr {
				if err == nil {
					t.Fatalf("Ожидалась ошибка, но получено nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Неожиданная ошибка: %v", err)
				}
				sortedNames := make([]string, len(sorted))
				for i, node := range sorted {
					sortedNames[i] = node.GetName()
				}
				if tc.name == "Нет зависимостей" {
					if len(sortedNames) != len(tc.expectedOrder) {
						t.Errorf("Ожидалось %d узлов, получено %d", len(tc.expectedOrder), len(sortedNames))
					}
				} else if !reflect.DeepEqual(sortedNames, tc.expectedOrder) {
					t.Errorf("Неправильный порядок сортировки.\nОжидалось: %v\nПолучено:  %v", tc.expectedOrder, sortedNames)
				}
			}
		})
	}
}
