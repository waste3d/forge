package orchestrator

import (
	"context"

	"github.com/waste3d/forge/pkg/parser"
)

type Node interface {
	GetName() string
	GetDependencies() []string
	Start(ctx context.Context, networkID string, orchestrator *Orchestrator) error
	IsReady(ctx context.Context, orchestrator *Orchestrator) error
}

type ServiceNode struct {
	*parser.ServiceConfig
	port int
}

func (s *ServiceNode) GetName() string {
	return s.Name
}

func (s *ServiceNode) GetDependencies() []string {
	return s.DependsOn
}

func (s *ServiceNode) Start(ctx context.Context, networkID string, orchestrator *Orchestrator) error {
	return orchestrator.startService(ctx, s.ServiceConfig, networkID)
}

func (s *ServiceNode) IsReady(ctx context.Context, orchestrator *Orchestrator) error {
	return orchestrator.healthCheckPort(ctx, s.Name, s.port)
}

type DBNode struct {
	*parser.DBConfig
	port int
}

func (d *DBNode) GetName() string {
	return d.Name
}

func (d *DBNode) GetDependencies() []string {
	if d.DependsOn == nil {
		return []string{}
	}
	return d.DependsOn
}

func (d *DBNode) Start(ctx context.Context, networkID string, orchestrator *Orchestrator) error {
	return orchestrator.startDatabase(ctx, d.DBConfig, networkID)
}

func (d *DBNode) IsReady(ctx context.Context, orchestrator *Orchestrator) error {
	return orchestrator.healthCheckPort(ctx, d.Name, d.port)
}
