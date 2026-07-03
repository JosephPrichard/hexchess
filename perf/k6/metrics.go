package main

import (
	"log/slog"
	"sync"
	"time"
)

type Metric struct {
	InputTime int64          `js:"inputTime"`
	InputType string         `js:"inputType"`
	Outputs   []OutputMetric `js:"outputs"`
}

type OutputMetric struct {
	OutputTime int64  `js:"outputTime"`
	OutputType string `js:"outputType"`
}

type Metrics struct {
	Lock      sync.Mutex
	MetricMap map[string]Metric
}

func NewMetricMap() *Metrics {
	return &Metrics{
		MetricMap: make(map[string]Metric),
	}
}

func (m *Metrics) NewMetric(messageId string, inputType string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()

	m.MetricMap[messageId] = Metric{
		InputTime: time.Now().UnixMilli(),
		InputType: inputType,
		Outputs:   []OutputMetric{},
	}
}

func (m *Metrics) AppendMetric(messageId string, outputType string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()

	metric, ok := m.MetricMap[messageId]
	if !ok {
		slog.Info("messageId not found in metrics map", "messageId", messageId)
		return
	}
	metric.Outputs = append(metric.Outputs, OutputMetric{
		OutputTime: time.Now().UnixMilli(),
		OutputType: outputType,
	})
	m.MetricMap[messageId] = metric
}
