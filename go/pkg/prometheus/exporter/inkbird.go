package exporter

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/pkg/ble/inkbird"
)

const inkbirdNamespace = "inkbird"

type InkBird struct {
	ids map[string]struct{}

	lastSeen    *prometheus.Desc
	temperature *prometheus.Desc
	humidity    *prometheus.Desc
	battery     *prometheus.Desc
	rssi        *prometheus.Desc
}

func NewInkBirdExporter(ctx context.Context, ids []string) (*InkBird, error) {
	err := inkbird.DefaultThermometerDataProvider.Start(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	id := make(map[string]struct{})
	for _, v := range ids {
		id[v] = struct{}{}
	}
	return &InkBird{
		ids: id,
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
		rssi: prometheus.NewDesc(
			prometheus.BuildFQName(inkbirdNamespace, "", "rssi"),
			"",
			[]string{"addr"},
			nil,
		),
	}, nil
}

func (e *InkBird) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.lastSeen
	ch <- e.temperature
	ch <- e.humidity
	ch <- e.battery
	ch <- e.rssi
}

func (e *InkBird) Collect(ch chan<- prometheus.Metric) {
	for id := range e.ids {
		data := inkbird.DefaultThermometerDataProvider.Get(id)
		if data == nil {
			return
		}

		ch <- prometheus.MustNewConstMetric(e.lastSeen, prometheus.CounterValue, float64(data.Time.Unix()), id)
		ch <- prometheus.MustNewConstMetric(e.temperature, prometheus.GaugeValue, float64(data.Temperature), id)
		ch <- prometheus.MustNewConstMetric(e.humidity, prometheus.GaugeValue, float64(data.Humidity), id)
		ch <- prometheus.MustNewConstMetric(e.battery, prometheus.GaugeValue, float64(data.Battery), id)
		ch <- prometheus.MustNewConstMetric(e.rssi, prometheus.GaugeValue, float64(data.RSSI), id)
	}
}

func (e *InkBird) Shutdown() error {
	return inkbird.DefaultThermometerDataProvider.Stop()
}
