// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package k8swindows

import (
	"context"
	"errors"
	"os"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type K8sWindows struct {
	cancel          context.CancelFunc
	logger          *zap.Logger
	nodeName        string `toml:"node_name"`
	k8sDecorator    stores.K8sDecorator
	summaryProvider *kubeletSummaryProvider
	hostInfo        host.Info
}

func New(logger *zap.Logger, decorator *stores.K8sDecorator, hostInfo host.Info) (*K8sWindows, error) {
	nodeName := os.Getenv("HOST_NAME")
	if nodeName == "" {
		return nil, errors.New("missing environment variable HOST_NAME. Please check your deployment YAML config")
	}
	k8sSummaryProvider, err := new(logger, hostInfo)
	if err != nil {
		logger.Error("failed to initialize kubelet summary provider, ", zap.Error(err))
		return nil, err
	}
	return &K8sWindows{
		logger:          logger,
		nodeName:        nodeName,
		k8sDecorator:    *decorator,
		summaryProvider: k8sSummaryProvider,
		hostInfo:        hostInfo,
	}, nil
}

func (k *K8sWindows) GetMetrics() []pmetric.Metrics {
	k.logger.Debug("D! called K8sWindows GetMetrics")
	var result []pmetric.Metrics

	metrics, err := k.summaryProvider.getMetrics()
	if err != nil {
		k.logger.Error("error getting metrics from kubelet summary provider, ", zap.Error(err))
		return result
	}
	metrics = k.decorateMetrics(metrics)
	for _, k8sSummaryMetric := range metrics {
		md := ci.ConvertToOTLPMetrics(k8sSummaryMetric.GetFields(), k8sSummaryMetric.GetTags(), k.logger)
		result = append(result, md)
	}

	return result
}

func (c *K8sWindows) decorateMetrics(cadvisormetrics []*extractors.CAdvisorMetric) []*extractors.CAdvisorMetric {
	//ebsVolumeIdsUsedAsPV := c.hostInfo.ExtractEbsIDsUsedByKubernetes()
	var result []*extractors.CAdvisorMetric
	for _, m := range cadvisormetrics {
		tags := m.GetTags()
		//c.addEbsVolumeInfo(tags, ebsVolumeIdsUsedAsPV)

		// add version
		//tags[ci.Version] = c.version

		// add nodeName for node, pod and container
		metricType := tags[ci.MetricType]
		if c.nodeName != "" && (ci.IsNode(metricType) || ci.IsInstance(metricType) ||
			ci.IsPod(metricType) || ci.IsContainer(metricType)) {
			tags[ci.NodeNameKey] = c.nodeName
		}

		// add instance id and type
		if instanceID := c.hostInfo.GetInstanceID(); instanceID != "" {
			tags[ci.InstanceID] = instanceID
		}
		if instanceType := c.hostInfo.GetInstanceType(); instanceType != "" {
			tags[ci.InstanceType] = instanceType
		}

		// add scaling group name
		tags[ci.AutoScalingGroupNameKey] = c.hostInfo.GetAutoScalingGroupName()

		// add tags for EKS
		tags[ci.ClusterNameKey] = c.hostInfo.GetClusterName()

		out := c.k8sDecorator.Decorate(m)
		if out != nil {
			result = append(result, out)
		}
	}
	return result
}

func (k *K8sWindows) Shutdown() error {
	k.logger.Debug("D! called K8sWindows Shutdown")
	return nil
}
