package perf

import (
	"errors"

	"github.com/montanaflynn/stats"
)

type GrafanaTrend struct {
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	Med float64 `json:"med"`
	Max float64 `json:"max"`
	P90 float64 `json:"p(90)"`
	P95 float64 `json:"p(95)"`
	P99 float64 `json:"p(99)"`
}

type GrafanaMetric struct {
	Name     string
	Type     string       `json:"type"`
	Contains string       `json:"contains"`
	Values   GrafanaTrend `json:"values"`
}

type GrafanaSummary struct {
	Metrics map[string]GrafanaMetric `json:"metrics"`
}

func BuildGrafanaTrend(latenciesMs []float64) (GrafanaTrend, error) {
	data := stats.LoadRawData(latenciesMs)

	var errs error

	avg, err := stats.Mean(data)
	errs = errors.Join(errs, err)

	min, err := stats.Min(data)
	errs = errors.Join(errs, err)

	med, err := stats.Median(data)
	errs = errors.Join(errs, err)

	max, err := stats.Max(data)
	errs = errors.Join(errs, err)

	p90, err := stats.Percentile(data, 90)
	errs = errors.Join(errs, err)

	p95, err := stats.Percentile(data, 95)
	errs = errors.Join(errs, err)

	p99, err := stats.Percentile(data, 99)
	errs = errors.Join(errs, err)

	return GrafanaTrend{
		Avg: avg,
		Min: min,
		Med: med,
		Max: max,
		P90: p90,
		P95: p95,
		P99: p99,
	}, errs
}
