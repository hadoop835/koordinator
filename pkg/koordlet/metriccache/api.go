/*
Copyright 2022 The Koordinator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metriccache

import (
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/koordinator-sh/koordinator/pkg/koordlet/util"
)

type CPUMetric struct {
	CPUUsed resource.Quantity
}

type MemoryMetric struct {
	MemoryWithoutCache resource.Quantity
}

type CPUThrottledMetric struct {
	ThrottledRatio float64
}

type NodeResourceMetric struct {
	CPUUsed    CPUMetric
	MemoryUsed MemoryMetric
}

type NodeResourceQueryResult struct {
	QueryResult
	Metric *NodeResourceMetric
}

type PodResourceMetric struct {
	PodUID     string
	CPUUsed    CPUMetric
	MemoryUsed MemoryMetric
}

type PodResourceQueryResult struct {
	QueryResult
	Metric *PodResourceMetric
}

type ContainerResourceMetric struct {
	ContainerID string
	CPUUsed     CPUMetric
	MemoryUsed  MemoryMetric
}

type ContainerResourceQueryResult struct {
	QueryResult
	Metric *ContainerResourceMetric
}

type NodeCPUInfo util.LocalCPUInfo

type BECPUResourceMetric struct {
	CPUUsed      resource.Quantity //cpuUsed cores for BestEffort Cgroup
	CPURealLimit resource.Quantity //suppressCPUQuantity: if suppress by cfs_quota then this  value is cfs_quota/cfs_period
	CPURequest   resource.Quantity //sum(extendResources_Cpu:request) by all qos:BE pod
}

type BECPUResourceQueryResult struct {
	QueryResult
	Metric *BECPUResourceMetric
}

type PodThrottledMetric struct {
	PodUID             string
	CPUThrottledMetric *CPUThrottledMetric
}

type ContainerThrottledMetric struct {
	ContainerID        string
	CPUThrottledMetric *CPUThrottledMetric
}

type PodThrottledQueryResult struct {
	QueryResult
	Metric *PodThrottledMetric
}

type ContainerThrottledQueryResult struct {
	QueryResult
	Metric *ContainerThrottledMetric
}
