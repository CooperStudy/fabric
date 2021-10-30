/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package prometheus

import (
	"fmt"
	kitmetrics "github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/hyperledger/fabric/common/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

type Provider struct{}

func (p *Provider) NewCounter(o metrics.CounterOpts) metrics.Counter {
	fmt.Println("===Provider========NewCounter================")
	return &Counter{
		Counter: prometheus.NewCounterFrom(
			prom.CounterOpts{
				Namespace: o.Namespace,
				Subsystem: o.Subsystem,
				Name:      o.Name,
				Help:      o.Help,
			},
			o.LabelNames,
		),
	}
}

func (p *Provider) NewGauge(o metrics.GaugeOpts) metrics.Gauge {
	fmt.Println("===Provider========NewGauge================")
	return &Gauge{
		Gauge: prometheus.NewGaugeFrom(
			prom.GaugeOpts{
				Namespace: o.Namespace,
				Subsystem: o.Subsystem,
				Name:      o.Name,
				Help:      o.Help,
			},
			o.LabelNames,
		),
	}
}

func (p *Provider) NewHistogram(o metrics.HistogramOpts) metrics.Histogram {
	fmt.Println("===Provider========NewHistogram================")
	return &Histogram{
		Histogram: prometheus.NewHistogramFrom(
			prom.HistogramOpts{
				Namespace: o.Namespace,
				Subsystem: o.Subsystem,
				Name:      o.Name,
				Help:      o.Help,
				Buckets:   o.Buckets,
			},
			o.LabelNames,
		),
	}
}

type Counter struct{ kitmetrics.Counter }

func (c *Counter) With(labelValues ...string) metrics.Counter {
	fmt.Println("===Counter========With================")
	return &Counter{Counter: c.Counter.With(labelValues...)}
}

type Gauge struct{ kitmetrics.Gauge }

func (g *Gauge) With(labelValues ...string) metrics.Gauge {
	fmt.Println("===Gauge========With================")
	return &Gauge{Gauge: g.Gauge.With(labelValues...)}
}

type Histogram struct{ kitmetrics.Histogram }

func (h *Histogram) With(labelValues ...string) metrics.Histogram {
	fmt.Println("===Histogram========With================")
	return &Histogram{Histogram: h.Histogram.With(labelValues...)}
}
