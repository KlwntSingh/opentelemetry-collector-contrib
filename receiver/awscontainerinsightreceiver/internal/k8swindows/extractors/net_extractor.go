// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"time"

	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
)

type NetMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (n *NetMetricExtractor) HasValue(rawMetric RawMetric) bool {
	if !rawMetric.Time.IsZero() {
		return true
	}
	return false
}

func (n *NetMetricExtractor) GetValue(rawMetric RawMetric, mInfo cExtractor.CPUMemInfoProvider, containerType string) []*cExtractor.CAdvisorMetric {
	var metrics []*cExtractor.CAdvisorMetric

	if containerType == ci.TypeContainer {
		return nil
	}

	netIfceMetrics := make([]map[string]any, len(rawMetric.NetworkStats))

	for i, intf := range rawMetric.NetworkStats {
		netIfceMetric := make(map[string]any)

		identifier := rawMetric.Id + containerType + intf.Name
		multiplier := float64(time.Second)

		cExtractor.AssignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetRxBytes, identifier, float64(intf.RxBytes), rawMetric.Time, multiplier)
		cExtractor.AssignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetRxErrors, identifier, float64(intf.RxErrors), rawMetric.Time, multiplier)
		cExtractor.AssignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetTxBytes, identifier, float64(intf.TxBytes), rawMetric.Time, multiplier)
		cExtractor.AssignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetTxErrors, identifier, float64(intf.TxErrors), rawMetric.Time, multiplier)

		if netIfceMetric[ci.NetRxBytes] != nil && netIfceMetric[ci.NetTxBytes] != nil {
			netIfceMetric[ci.NetTotalBytes] = netIfceMetric[ci.NetRxBytes].(float64) + netIfceMetric[ci.NetTxBytes].(float64)
		}

		netIfceMetrics[i] = netIfceMetric
	}

	aggregatedFields := ci.SumFields(netIfceMetrics)
	if len(aggregatedFields) > 0 {
		metric := cExtractor.NewCadvisorMetric(containerType, n.logger)
		for k, v := range aggregatedFields {
			metric.AddField(ci.MetricName(containerType, k), v)
		}
		metrics = append(metrics, metric)
	}

	return metrics
}

func (n *NetMetricExtractor) Shutdown() error {
	return n.rateCalculator.Shutdown()
}

func NewNetMetricExtractor(logger *zap.Logger) *NetMetricExtractor {
	return &NetMetricExtractor{
		logger:         logger,
		rateCalculator: cExtractor.NewFloat64RateCalculator(),
	}
}

func getNetMetricType(containerType string, logger *zap.Logger) string {
	metricType := ""
	switch containerType {
	case ci.TypeNode:
		metricType = ci.TypeNodeNet
	case ci.TypePod:
		metricType = ci.TypePodNet
	default:
		logger.Warn("net_extractor: net metric extractor is parsing unexpected containerType", zap.String("containerType", containerType))
	}
	return metricType
}
