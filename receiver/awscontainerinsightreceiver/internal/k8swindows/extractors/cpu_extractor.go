package extractors

import (
	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"go.uber.org/zap"
)

const (
	decimalToMillicores = 1000
)

type CPUMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func (c *CPUMetricExtractor) HasValue(rawMetric *RawMetric) bool {
	if rawMetric.CPUStats != nil {
		return true
	}
	return false
}

func (c *CPUMetricExtractor) GetValue(rawMetric *RawMetric, mInfo cExtractor.CPUMemInfoProvider, containerType string) []*stores.RawContainerInsightsMetric {
	var metrics []*stores.RawContainerInsightsMetric

	metric := stores.NewRawContainerInsightsMetric(containerType, c.logger)

	multiplier := float64(decimalToMillicores)
	identifier := rawMetric.Id
	cExtractor.AssignRateValueToField(&c.rateCalculator, metric.GetFields(), ci.MetricName(containerType, ci.CPUTotal), identifier, float64(*rawMetric.CPUStats.UsageCoreNanoSeconds), rawMetric.Time, multiplier)

	numCores := mInfo.GetNumCores()
	if metric.GetField(ci.MetricName(containerType, ci.CPUTotal)) != nil && numCores != 0 {
		metric.AddField(ci.MetricName(containerType, ci.CPUUtilization), metric.GetField(ci.MetricName(containerType, ci.CPUTotal)).(float64)/float64(numCores*decimalToMillicores)*100)
	}

	if containerType == ci.TypeNode {
		metric.AddField(ci.MetricName(containerType, ci.CPULimit), numCores*decimalToMillicores)
	}

	metrics = append(metrics, metric)
	return metrics
}

func (c *CPUMetricExtractor) Shutdown() error {
	return c.rateCalculator.Shutdown()
}

func NewCPUMetricExtractor(logger *zap.Logger) *CPUMetricExtractor {
	return &CPUMetricExtractor{
		logger:         logger,
		rateCalculator: cExtractor.NewFloat64RateCalculator(),
	}
}
