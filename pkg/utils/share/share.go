// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package share

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/fields"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	liqostorage "github.com/liqotech/liqo/pkg/virtualKubelet/reflection/storage"
)

// Resources contains the resources quantities.
type Resources struct {
	CPU     resource.Quantity
	Memory  resource.Quantity
	Storage resource.Quantity
	Pods    resource.Quantity
}

// CPUToString returns the CPU quantity as a string.
func (r *Resources) CPUToString() string {
	cpu := r.CPU.ScaledValue(resource.Milli)
	return fmt.Sprintf("%dm", cpu)
}

// MemoryToString returns the memory quantity as a string.
func (r *Resources) MemoryToString() string {
	mem := float64(r.Memory.ScaledValue(resource.Mega)) / 1000
	return fmt.Sprintf("%.3fGB", mem)
}

// StorageToString returns the storage quantity as a string.
func (r *Resources) StorageToString() string {
	storage := float64(r.Storage.ScaledValue(resource.Mega)) / 1000
	return fmt.Sprintf("%.2fGB", storage)
}

// PodsToString returns the pods quantity as a string.
func (r *Resources) PodsToString() string {
	return r.Pods.String()
}

// GetIncomingTotal returns the total incoming resources for a given cluster.
func GetIncomingTotal(ctx context.Context, cl client.Client, clusterID string) (Resources, error) {
	r, err := getters.GetResourceOfferByLabel(ctx, cl, metav1.NamespaceAll, liqolabels.RemoteLabelSelectorForCluster(clusterID))
	if err != nil {
		return Resources{}, err
	}
	return Resources{
		CPU:     *r.Spec.ResourceQuota.Hard.Cpu(),
		Memory:  *r.Spec.ResourceQuota.Hard.Memory(),
		Storage: *r.Spec.ResourceQuota.Hard.StorageEphemeral(),
		Pods:    *r.Spec.ResourceQuota.Hard.Pods(),
	}, nil
}

// GetIncomingUsed returns the used incoming resources for a given cluster.
func GetIncomingUsed(ctx context.Context, cl client.Client, clusterID string) (Resources, error) {
	node, err := getters.GetNodeByClusterID(ctx, cl, &v1alpha1.ClusterIdentity{ClusterID: clusterID})
	if err != nil {
		return Resources{}, err
	}

	pods, err := getters.ListOffloadedPodsByNode(ctx, cl, corev1.NamespaceAll, node.Name)
	if err != nil {
		return Resources{}, err
	}

	var podMetricsFound metricsv1beta1.PodMetricsList
	for i := range pods.Items {
		podMetrics, err := getters.GetPodMetricsByField(ctx, cl, fields.NameAndNamespaceFieldSelector(pods.Items[i].Name, pods.Items[i].Namespace))
		if err != nil {
			return Resources{}, err
		}
		podMetricsFound.Items = append(podMetricsFound.Items, podMetrics.Items...)
	}

	cpu, mem := aggregatePodsMetrics(podMetricsFound.Items)

	pvcs, err := getters.GetPVCByLabel(ctx, cl, labels.NewSelector())
	if err != nil {
		return Resources{}, err
	}

	filteredPvcs := filterOutgoingLiqoPVCsByNode(pvcs.Items, node.Name)

	storage := aggregatePVCsStorage(filteredPvcs)

	return Resources{
		CPU:     cpu,
		Memory:  mem,
		Storage: storage,
		Pods:    *resource.NewQuantity(int64(len(pods.Items)), resource.DecimalSI),
	}, nil
}

// GetOutgoingTotal returns the total outgoing resources for a given cluster.
func GetOutgoingTotal(ctx context.Context, cl client.Client, clusterID string) (Resources, error) {
	r, err := getters.GetResourceOfferByLabel(ctx, cl, metav1.NamespaceAll, liqolabels.LocalLabelSelectorForCluster(clusterID))
	if err != nil {
		return Resources{}, err
	}
	return Resources{
		CPU:     *r.Spec.ResourceQuota.Hard.Cpu(),
		Memory:  *r.Spec.ResourceQuota.Hard.Memory(),
		Storage: *r.Spec.ResourceQuota.Hard.StorageEphemeral(),
		Pods:    *r.Spec.ResourceQuota.Hard.Pods(),
	}, nil
}

// GetOutgoingUsed returns the used outgoing resources for a given cluster.
func GetOutgoingUsed(ctx context.Context, cl client.Client, clusterID string) (Resources, error) {
	pml, err := getters.GetPodMetricsByLabel(ctx, cl, liqolabels.LiqoOriginLabelSelector(clusterID))
	if err != nil {
		return Resources{}, err
	}
	cpu, mem := aggregatePodsMetrics(pml.Items)

	pvcs, err := getters.GetPVCByLabel(ctx, cl, liqolabels.LiqoOriginLabelSelector(clusterID))
	if err != nil {
		return Resources{}, err
	}

	storage := aggregatePVCsStorage(pvcs.Items)

	return Resources{
		CPU:     cpu,
		Memory:  mem,
		Storage: storage,
		Pods:    *resource.NewQuantity(int64(len(pml.Items)), resource.DecimalSI),
	}, nil
}

func aggregatePodsMetrics(podsMetrics []metricsv1beta1.PodMetrics) (cpu, mem resource.Quantity) {
	cpu = *resource.NewMilliQuantity(0, "DecimalSI")
	mem = *resource.NewMilliQuantity(0, "DecimalSI")

	for i := range podsMetrics {
		for _, container := range podsMetrics[i].Containers {
			if container.Usage.Cpu() != nil {
				cpu.Add(*container.Usage.Cpu())
			}
			if container.Usage.Memory() != nil {
				mem.Add(*container.Usage.Memory())
			}
		}
	}
	return cpu, mem
}

func aggregatePVCsStorage(pvcs []corev1.PersistentVolumeClaim) resource.Quantity {
	storage := *resource.NewMilliQuantity(0, "DecimalSI")

	for i := range pvcs {
		if pvcs[i].Status.Capacity.StorageEphemeral() != nil {
			storage.Add(*pvcs[i].Status.Capacity.Storage())
		}
	}
	return storage
}

func filterOutgoingLiqoPVCsByNode(pvcs []corev1.PersistentVolumeClaim, nodeName string) []corev1.PersistentVolumeClaim {
	var filteredPvcs []corev1.PersistentVolumeClaim
	for i := range pvcs {
		if pvcs[i].ObjectMeta.Annotations[liqostorage.AnnSelectedNode] == nodeName &&
			pvcs[i].ObjectMeta.Annotations[liqostorage.AnnStorageProvisioner] == liqoconsts.StorageProvisionerName {
			filteredPvcs = append(filteredPvcs, pvcs[i])
		}
	}
	return filteredPvcs
}
