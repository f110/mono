package exporter

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.f110.dev/mono/go/pkg/ble/inkbird"
	"go.f110.dev/mono/go/pkg/logger"
	"go.uber.org/zap"
)

const inkbirdNamespace = "inkbird"

type InkBird struct {
	id              string
	minimumInterval time.Duration

	lastSeen    *prometheus.Desc
	temperature *prometheus.Desc
	humidity    *prometheus.Desc
	battery     *prometheus.Desc

	lastData *inkbird.ThermometerData
}

func NewInkBirdExporter(id string, minimumInterval time.Duration) *InkBird {
	return &InkBird{
		id:              id,
		minimumInterval: minimumInterval,
		lastSeen: prometheus.NewDesc(
			prometheus.BuildFQName(inkbirdNamespace, "", "last_seen"),
			"",
			[]string{"addr"},
			nil,
		),
		temperature: prometheus.NewDesc(
			prometheus.BuildFQName(inkbirdNamespace, "", "temperature"),
			"",
			[]string{"addr"},
			nil,
		),
		humidity: prometheus.NewDesc(
			prometheus.BuildFQName(inkbirdNamespace, "", "humidity"),
			"",
			[]string{"addr"},
			nil,
		),
		battery: prometheus.NewDesc(
			prometheus.BuildFQName(inkbirdNamespace, "", "battery"),
			"",
			[]string{"addr"},
			nil,
		),
	}
}

func (e *InkBird) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.lastSeen
	ch <- e.temperature
	ch <- e.humidity
	ch <- e.battery
}

func (e *InkBird) Collect(ch chan<- prometheus.Metric) {
	if e.lastData != nil && time.Now().Before(e.lastData.Time.Add(e.minimumInterval)) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	logger.Log.Debug("The cache is expired. Get from bluetooth")
	data, err := inkbird.Read(ctx, e.id)
	if err != nil {
		logger.Log.Warn("Failed to read data", zap.Error(err))
		return
	}
	e.lastData = data

	ch <- prometheus.MustNewConstMetric(e.lastSeen, prometheus.CounterValue, float64(data.Time.Unix()), e.id)
	ch <- prometheus.MustNewConstMetric(e.temperature, prometheus.GaugeValue, float64(data.Temperature), e.id)
	ch <- prometheus.MustNewConstMetric(e.humidity, prometheus.GaugeValue, float64(data.Humidity), e.id)
	ch <- prometheus.MustNewConstMetric(e.battery, prometheus.GaugeValue, float64(data.Battery), e.id)
}
