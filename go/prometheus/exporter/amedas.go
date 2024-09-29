package exporter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.f110.dev/mono/go/jma"
)

const jmaNamespace = "jma"

type target struct {
	Pref jma.PrefectureNumber
	Site int
}

type Amedas struct {
	targets          []*target
	cache            *amedasDataCache
	lastObservedTime *prometheus.Desc
	temperature      *prometheus.Desc
	humidity         *prometheus.Desc
	pressure         *prometheus.Desc
}

var _ prometheus.Collector = (*Amedas)(nil)

func NewAmedasExporter(targets [][]int) *Amedas {
	var t []*target
	for _, v := range targets {
		t = append(t, &target{Pref: jma.PrefectureNumber(v[0]), Site: v[1]})
	}
	return &Amedas{
		targets: t,
		cache:   newAmedasDataCache(),
		lastObservedTime: prometheus.NewDesc(
			prometheus.BuildFQName(jmaNamespace, "amedas", "last_observed_time"),
			"",
			[]string{"pref", "site"},
			nil,
		),
		temperature: prometheus.NewDesc(
			prometheus.BuildFQName(jmaNamespace, "amedas", "temperature"),
			"",
			[]string{"pref", "site"},
			nil,
		),
		humidity: prometheus.NewDesc(
			prometheus.BuildFQName(jmaNamespace, "amedas", "humidity"),
			"",
			[]string{"pref", "site"},
			nil,
		),
		pressure: prometheus.NewDesc(
			prometheus.BuildFQName(jmaNamespace, "amedas", "pressure"),
			"",
			[]string{"pref", "site"},
			nil,
		),
	}
}

func (a *Amedas) Describe(ch chan<- *prometheus.Desc) {
	ch <- a.lastObservedTime
	ch <- a.temperature
	ch <- a.humidity
	ch <- a.pressure
}

func (a *Amedas) Collect(ch chan<- prometheus.Metric) {
	for _, t := range a.targets {
		data, err := a.cache.Get(context.Background(), t.Pref, t.Site)
		if err != nil {
			continue
		}
		pref, site := fmt.Sprintf("%d", t.Pref), fmt.Sprintf("%d", t.Site)

		ch <- prometheus.MustNewConstMetric(a.lastObservedTime, prometheus.GaugeValue, float64(data.Time.Unix()), pref, site)
		ch <- prometheus.MustNewConstMetric(a.temperature, prometheus.GaugeValue, data.Temperature, pref, site)
		ch <- prometheus.MustNewConstMetric(a.humidity, prometheus.GaugeValue, float64(data.Humidity), pref, site)
		ch <- prometheus.MustNewConstMetric(a.pressure, prometheus.GaugeValue, data.Pressure, pref, site)
	}
}

type amedasDataCache struct {
	client *jma.Amedas
	data   map[string][]*jma.AmedasData
	lock   map[string]*sync.Mutex
}

func newAmedasDataCache() *amedasDataCache {
	return &amedasDataCache{client: jma.NewAmedas(), data: make(map[string][]*jma.AmedasData), lock: make(map[string]*sync.Mutex)}
}

func (c *amedasDataCache) Get(ctx context.Context, pref jma.PrefectureNumber, site int) (*jma.AmedasData, error) {
	t := time.Now()
	key := fmt.Sprintf("%d%d", pref, site)
	if _, ok := c.lock[key]; !ok {
		c.lock[key] = new(sync.Mutex)
	}
	c.lock[key].Lock()
	defer c.lock[key].Unlock()

	data, ok := c.data[key]
	if !ok {
		var err error
		data, err = c.client.GetObservedData(ctx, pref, site, t)
		if err != nil {
			return nil, err
		}
	}
	if t.Before(data[0].Time) {
		var err error
		data, err = c.client.GetObservedData(ctx, pref, site, t)
		if err != nil {
			return nil, err
		}
	}
	c.data[key] = data

	for i := len(data) - 1; i >= 0; i-- {
		if t.Before(data[i].Time) && i+1 < len(data) {
			return data[i+1], nil
		}
	}
	return nil, nil
}
