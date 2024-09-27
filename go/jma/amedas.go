package jma

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"sort"
	"time"

	"go.f110.dev/xerrors"
)

type Amedas struct {
	client *http.Client
}

func NewAmedas() *Amedas {
	return &Amedas{
		client: http.DefaultClient,
	}
}

func (a *Amedas) GetObservedData(ctx context.Context, pref PrefectureNumber, site int, t time.Time) ([]*AmedasData, error) {
	truncated := t.Truncate(3 * time.Hour)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://www.jma.go.jp/bosai/amedas/data/point/%d%d/%d%02d%02d_%02d.json", pref, site, truncated.Year(), truncated.Month(), truncated.Day(), truncated.Hour()),
		nil,
	)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	default:
		return nil, xerrors.Newf("got response code %d", res.StatusCode)
	}

	var data amedasDataPoint
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, xerrors.WithStack(err)
	}
	var result []*AmedasData
	for dts, v := range data {
		dt, err := time.Parse("20060102150400", dts)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		var pressure, seeLevelPressure, temperature float64
		var humidity int
		if len(v.Pressure) > 1 {
			pressure = v.Pressure[0].(float64)
		}
		if len(v.NormalPressure) > 1 {
			seeLevelPressure = v.NormalPressure[0].(float64)
		}
		if len(v.Temperature) > 1 {
			temperature = v.Temperature[0].(float64)
		}
		if len(v.Humidity) > 1 {
			humidity = v.Humidity[0]
		}

		result = append(result, &AmedasData{
			Time:             dt,
			Pressure:         pressure,
			SeeLevelPressure: seeLevelPressure,
			Humidity:         humidity,
			Temperature:      temperature,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Time.Before(result[j].Time) })
	return result, nil
}

// GetObservedDataRange returns a list of AmedasData.
// The list is sorted in ascending order, oldest first.
func (a *Amedas) GetObservedDataRange(ctx context.Context, pref PrefectureNumber, site int, t time.Time, d time.Duration) ([]*AmedasData, error) {
	earliestTime := t.Add(-d).Truncate(3 * time.Hour)
	baseTime := earliestTime
	var result []*AmedasData
	for {
		data, err := a.GetObservedData(ctx, pref, site, baseTime)
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
		baseTime = baseTime.Add(3 * time.Hour)
		if baseTime.After(t) {
			break
		}
	}
	return result, nil
}

// GetObservedDataIter returns the iterator for AmedasData.
// Data is returned in descending order.
func (a *Amedas) GetObservedDataIter(ctx context.Context, pref PrefectureNumber, site int, t time.Time) iter.Seq[*AmedasDataIter] {
	baseTime := t.Truncate(3 * time.Hour)
	return func(yield func(*AmedasDataIter) bool) {
		for {
			data, err := a.GetObservedData(ctx, pref, site, baseTime)
			if err != nil {
				yield(&AmedasDataIter{Err: err})
				return
			}
			for i := len(data) - 1; i > 0; i-- {
				if !yield(&AmedasDataIter{AmedasData: data[i]}) {
					return
				}
			}
			baseTime = baseTime.Add(-3 * time.Hour)
		}
	}
}

type AmedasData struct {
	Time             time.Time
	SeeLevelPressure float64
	Pressure         float64
	Temperature      float64
	Humidity         int
}

type AmedasDataIter struct {
	*AmedasData
	Err error
}

type amedasDataPoint map[string]*amedasData

type amedasData struct {
	PrefNumber        int       `json:"prefNumber"`
	ObservationNumber int       `json:"observationNumber"`
	Pressure          []any     `json:"pressure"`       // atmospheric pressure
	NormalPressure    []any     `json:"normalPressure"` // see-level pressure
	Temperature       []any     `json:"temp"`
	Humidity          []int     `json:"humidity"`
	Snow              []int     `json:"snow"`
	Snow1h            []int     `json:"snow1h"`
	Snow6h            []int     `json:"snow6h"`
	Snow12h           []int     `json:"snow12h"`
	Snow24h           []int     `json:"snow24h"`
	Sun10m            []int     `json:"sun10m"`
	Sun1h             []any     `json:"sun1h"`
	Precipitation10m  []any     `json:"precipitation10m"`
	Precipitation1h   []any     `json:"precipitation1h"`
	Precipitation3h   []any     `json:"precipitation3h"`
	Precipitation24h  []any     `json:"precipitation24h"`
	WindDirection     []int     `json:"windDirection"`
	Wind              []any     `json:"wind"`
	MaxTempTime       *dataTime `json:"maxTempTime"`
	MaxTemp           []any     `json:"maxTemp"`
	MinTempTime       *dataTime `json:"minTempTime"`
	MinTemp           []any     `json:"minTemp"`
	GustTime          *dataTime `json:"gustTime"`
	GustDirection     []int     `json:"gustDirection"`
	Gust              []any     `json:"gust"`
}

type dataTime struct {
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}
