package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/waste3d/forge/internal/state"
	"github.com/waste3d/forge/metrics"
)

func RunMetricsCollector(ctx context.Context, logger *slog.Logger, cli *client.Client, sm *state.Manager) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	logger.Info("Запущен сборщик метрик...")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Остановка сборщика метрик...")
			return
		case <-ticker.C:
			logger.Debug("Обновление метрик...")
			if err := updateAllMetrics(ctx, cli, sm); err != nil {
				logger.Error("Ошибка при обновлении метрик", "error", err)
			}
		}
	}
}

func updateAllMetrics(ctx context.Context, cli *client.Client, sm *state.Manager) error {
	resources, err := sm.GetAllResources()
	if err != nil {
		return err
	}

	metrics.ServiceStatus.Reset()
	metrics.ServiceCPUUsage.Reset()
	metrics.ServiceMemoryUsage.Reset()

	apps := make(map[string]struct{})
	serviceCount := 0

	for _, res := range resources {
		if res.ResourceType != "container" {
			continue
		}

		apps[res.AppName] = struct{}{}
		serviceCount++

		inspect, err := cli.ContainerInspect(ctx, res.ID)
		if err != nil {
			if client.IsErrNotFound(err) {
				metrics.ServiceStatus.WithLabelValues(res.AppName, res.ServiceName).Set(0)
				continue
			}
			return err
		}

		if inspect.State.Running {
			metrics.ServiceStatus.WithLabelValues(res.AppName, res.ServiceName).Set(1)
			updateContainerStats(ctx, cli, res)
		} else {
			metrics.ServiceStatus.WithLabelValues(res.AppName, res.ServiceName).Set(0)
		}
	}

	metrics.ManagedApps.Set(float64(len(apps)))
	metrics.ManagedServices.Set(float64(serviceCount))

	return nil
}

func updateContainerStats(ctx context.Context, cli *client.Client, res state.Resource) {
	stats, err := cli.ContainerStats(ctx, res.ID, false) // false = не стримить, получить один раз
	if err != nil {
		return
	}
	defer stats.Body.Close()

	body, err := io.ReadAll(stats.Body)
	if err != nil {
		return
	}

	var v *types.StatsJSON
	if err := json.Unmarshal(body, &v); err != nil {
		return
	}

	// Расчет CPU
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Память
	memUsage := float64(v.MemoryStats.Usage-v.MemoryStats.Stats["cache"]) / (1024 * 1024) // в MiB

	metrics.ServiceCPUUsage.WithLabelValues(res.AppName, res.ServiceName, res.ID[:12]).Set(cpuPercent)
	metrics.ServiceMemoryUsage.WithLabelValues(res.AppName, res.ServiceName, res.ID[:12]).Set(memUsage)
}
