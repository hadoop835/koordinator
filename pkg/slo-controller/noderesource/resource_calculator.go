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

package noderesource

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"

	"github.com/koordinator-sh/koordinator/apis/extension"
	slov1alpha1 "github.com/koordinator-sh/koordinator/apis/slo/v1alpha1"
	"github.com/koordinator-sh/koordinator/pkg/slo-controller/config"
	"github.com/koordinator-sh/koordinator/pkg/util"
)

// calculateBEResource calculate BE resource using the formula below
// Node.Total - Node.Reserved - System.Used - Pod(LS).Used, System.Used = Node.Used - Pod(All).Used
func (r *NodeResourceReconciler) calculateBEResource(node *corev1.Node,
	podList *corev1.PodList, nodeMetric *slov1alpha1.NodeMetric) *nodeBEResource {
	// NOTE: for pod usage calculation, currently non-BE pods are considered as LS
	podLSUsed := util.NewZeroResourceList()
	// pod(All).Used = pod(LS).Used + pod(BE).Used
	podAllUsed := util.NewZeroResourceList()

	podMetricMap := make(map[string]*slov1alpha1.PodMetricInfo)
	for _, podMetric := range nodeMetric.Status.PodsMetric {
		podMetricMap[util.GetPodMetricKey(podMetric)] = podMetric
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		podKey := util.GetPodKey(&pod)
		podMetric, ok := podMetricMap[podKey]
		if !ok {
			podRequest := util.GetPodRequest(&pod, corev1.ResourceCPU, corev1.ResourceMemory)
			if extension.GetPodQoSClass(&pod) != extension.QoSBE {
				podLSUsed = quotav1.Add(podLSUsed, podRequest)
			}
			podAllUsed = quotav1.Add(podAllUsed, podRequest)
			continue
		}

		if extension.GetPodQoSClass(&pod) != extension.QoSBE {
			podLSUsed = quotav1.Add(podLSUsed, r.getPodMetricUsage(podMetric))
		}
		podAllUsed = quotav1.Add(podAllUsed, r.getPodMetricUsage(podMetric))
	}

	nodeAllocatable := r.getNodeAllocatable(node)
	nodeReservation := r.getNodeReservation(node)

	// System.Used = Node.Used - Pod(All).Used
	nodeUsage := r.getNodeMetricUsage(nodeMetric.Status.NodeMetric)
	systemUsed := quotav1.Max(quotav1.Subtract(nodeUsage, podAllUsed), util.NewZeroResourceList())

	// Node(BE).Alloc = Node.Total - Node.Reserved - System.Used - Pod(LS).Used; it's also equal to
	// Node.Total - Node.Reserved - Node.Usage + Pod(BE).Used, Pod(All).Used = Pod(BE).Used + Pod(LS).Used
	nodeAllocatableBE := quotav1.Max(quotav1.Subtract(
		quotav1.Subtract(quotav1.Subtract(
			nodeAllocatable, nodeReservation), systemUsed),
		podLSUsed), util.NewZeroResourceList())

	return &nodeBEResource{
		// transform cores into milli-cores
		MilliCPU:              resource.NewQuantity(nodeAllocatableBE.Cpu().MilliValue(), resource.DecimalSI),
		Memory:                nodeAllocatableBE.Memory(),
		IsColocationAvailable: true,
		Message: fmt.Sprintf(
			"nodeAllocatableBE[CPU(Milli-Core)]:%v = nodeAllocatable:%v - nodeReservation:%v - systemUsage:%v - podLSUsed:%v\n"+
				" nodeAllocatableBE[Mem(GB)]:%v = nodeAllocatable:%v - nodeReservation:%v - systemUsage:%v - podLSUsed:%v\n",
			nodeAllocatableBE.Cpu().MilliValue(),
			nodeAllocatable.Cpu().MilliValue(),
			nodeReservation.Cpu().MilliValue(),
			systemUsed.Cpu().MilliValue(),
			podLSUsed.Cpu().MilliValue(),
			nodeAllocatableBE.Memory().ScaledValue(resource.Giga),
			nodeAllocatable.Memory().ScaledValue(resource.Giga),
			nodeReservation.Memory().ScaledValue(resource.Giga),
			systemUsed.Memory().ScaledValue(resource.Giga),
			podLSUsed.Memory().ScaledValue(resource.Giga),
		),
	}
}

// getPodMetricUsage gets pod usage from the PodMetricInfo
func (r *NodeResourceReconciler) getPodMetricUsage(info *slov1alpha1.PodMetricInfo) corev1.ResourceList {
	cpuQuant := info.PodUsage.ResourceList[corev1.ResourceCPU]
	cpuUsageQuant := resource.NewMilliQuantity(cpuQuant.MilliValue(), cpuQuant.Format)
	memQuant := info.PodUsage.ResourceList[corev1.ResourceMemory]
	memUsageQuant := resource.NewQuantity(memQuant.Value(), memQuant.Format)
	return corev1.ResourceList{corev1.ResourceCPU: *cpuUsageQuant, corev1.ResourceMemory: *memUsageQuant}
}

// getNodeMetricUsage gets node usage from the NodeMetricInfo
func (r *NodeResourceReconciler) getNodeMetricUsage(info *slov1alpha1.NodeMetricInfo) corev1.ResourceList {
	cpuQuant := info.NodeUsage.ResourceList[corev1.ResourceCPU]
	cpuUsageQuant := resource.NewMilliQuantity(cpuQuant.MilliValue(), cpuQuant.Format)
	memQuant := info.NodeUsage.ResourceList[corev1.ResourceMemory]
	memUsageQuant := resource.NewQuantity(memQuant.Value(), memQuant.Format)
	return corev1.ResourceList{corev1.ResourceCPU: *cpuUsageQuant, corev1.ResourceMemory: *memUsageQuant}
}

// getNodeAllocatable gets node allocatable and filters out non-CPU and non-Mem resources
func (r *NodeResourceReconciler) getNodeAllocatable(node *corev1.Node) corev1.ResourceList {
	result := node.Status.Allocatable.DeepCopy()
	result = quotav1.Mask(result, []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory})
	return result
}

// getNodeReservation gets node-level safe-guarding reservation with the node's allocatable
func (r *NodeResourceReconciler) getNodeReservation(node *corev1.Node) corev1.ResourceList {
	r.config.RLock()
	defer r.config.RUnlock()

	strategy := config.GetNodeColocationStrategy(&r.config.ColocationCfg, node)

	nodeAllocatable := r.getNodeAllocatable(node)
	cpuReserveQuant := util.MultiplyMilliQuant(nodeAllocatable[corev1.ResourceCPU], r.getReserveRatio(*strategy.CPUReclaimThresholdPercent))
	memReserveQuant := util.MultiplyQuant(nodeAllocatable[corev1.ResourceMemory], r.getReserveRatio(*strategy.MemoryReclaimThresholdPercent))

	return corev1.ResourceList{
		corev1.ResourceCPU:    cpuReserveQuant,
		corev1.ResourceMemory: memReserveQuant,
	}
}

// getReserveRatio returns resource reserved ratio
func (r *NodeResourceReconciler) getReserveRatio(reclaimThreshold int64) float64 {
	return float64(100-reclaimThreshold) / 100.0
}
