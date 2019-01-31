package systems

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	. "github.com/Wizcorp/goal/src/api"
)

func init() {
	RegisterSystem(1, "metrics", NewMetrics())
}

type GoalMetrics interface {
	GoalSystem
	RegisterCounter(name string, help string) prometheus.Counter
	RegisterGauge(name string, help string) prometheus.Gauge
	RegisterHistogram(name string, help string) prometheus.Histogram
}

type metrics struct {
	Status   Status
	prefix   string
	registry *prometheus.Registry
}

func NewMetrics() *metrics {
	return &metrics{
		Status:   DownStatus,
		prefix:   "goal_",
		registry: prometheus.NewRegistry(),
	}
}

func (metrics *metrics) Setup(server GoalServer, config *GoalConfig) error {
	metricsPath := config.String("path", "/metrics")

	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.WithFields(LogFields{
		"subpath": metricsPath,
	}).Info("Setting up metrics system")

	registry := metrics.registry

	goCollector := prometheus.NewGoCollector()
	registry.Register(goCollector)

	processCollector := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})
	registry.Register(processCollector)

	handler := promhttp.InstrumentMetricHandler(
		registry,
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	)

	http := (*server.GetSystem("http")).(GoalHTTP)
	http.Handle(metricsPath, handler)

	metrics.Status = UpStatus

	return nil
}

func (metrics *metrics) Teardown(server GoalServer, config *GoalConfig) error {
	logger := (*server.GetSystem("logger")).(GoalLogger).GetInstance()
	logger.Info("Tearing down metrics system")
	metrics.Status = DownStatus

	return nil
}

func (metrics *metrics) GetStatus() Status {
	return UpStatus
}

func (metrics *metrics) RegisterCounter(name string, help string) prometheus.Counter {
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.prefix,
		Name:      name,
		Help:      help,
	})

	metrics.registry.Register(counter)

	return counter
}

func (metrics *metrics) RegisterGauge(name string, help string) prometheus.Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metrics.prefix,
		Name:      name,
		Help:      help,
	})

	metrics.registry.Register(gauge)

	return gauge
}

func (metrics *metrics) RegisterHistogram(name string, help string) prometheus.Histogram {
	histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metrics.prefix,
		Name:      name,
		Help:      help,
	})

	metrics.registry.Register(histogram)

	return histogram
}
