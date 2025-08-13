package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Общее количество приложений, управляемых Forge.
	ManagedApps = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "forge_managed_apps_total",
		Help: "The total number of applications currently managed by Forge.",
	})

	// Общее количество сервисов (контейнеров), управляемых Forge.
	ManagedServices = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "forge_managed_services_total",
		Help: "The total number of services (containers) currently managed by Forge.",
	})

	// Статус конкретного сервиса. Используем вектор, чтобы у каждого сервиса была своя метрика.
	ServiceStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forge_service_status",
		Help: "Status of a specific service (1 for up, 0 for down).",
	},
		[]string{"app_name", "service_name"},
	)

	ServiceCPUUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forge_service_cpu_usage_percent",
		Help: "Current CPU usage of a service as a percentage of the host's total CPU.",
	}, []string{"app_name", "service_name", "container_id"})

	ServiceMemoryUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forge_service_memory_usage_mb",
		Help: "Current memory usage of a service in MiB.",
	},
		[]string{"app_name", "service_name", "container_id"},
	)
)
