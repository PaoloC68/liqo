// Copyright 2019-2023 The Liqo Authors
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

package nsoffctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	virtualnodeutils "github.com/liqotech/liqo/pkg/utils/virtualnode"
)

func (r *NamespaceOffloadingReconciler) enforceClusterSelector(ctx context.Context, nsoff *offv1alpha1.NamespaceOffloading,
	clusterIDMap map[string]*virtualkubeletv1alpha1.NamespaceMap) error {
	virtualNodes, err := getters.ListVirtualNodesByLabels(ctx, r.Client, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to retrieve VirtualNodes: %w", err)
	}
	clusterIDs := getters.RetrieveClusterIDsFromVirtualNodes(virtualNodes)

	// If the number of virtual nodes does not match that of namespacemaps, there is something wrong in the cluster.
	if len(clusterIDs) != len(clusterIDMap) {
		return fmt.Errorf("number of VirtualNodes (%d) does not match that of NamespaceMaps (%d)",
			len(clusterIDs), len(clusterIDMap))
	}

	var returnErr error
	for i := range virtualNodes.Items {
		match, err := virtualnodeutils.MatchSelectorTerms(&virtualNodes.Items[i], &nsoff.Spec.ClusterSelector)
		if err != nil {
			r.Recorder.Eventf(nsoff, corev1.EventTypeWarning, "Invalid", "Invalid ClusterSelector: %v", err)
			// We end the processing here, as this error will be triggered for all the virtual nodes.
			return fmt.Errorf("invalid ClusterSelector: %w", err)
		}

		if match {
			if err = addDesiredMapping(ctx, r.Client, nsoff.Namespace, r.remoteNamespaceName(nsoff),
				clusterIDMap[virtualNodes.Items[i].Spec.ClusterIdentity.ClusterID]); err != nil {
				returnErr = fmt.Errorf("failed to configure all desired mappings")
			}
		} else {
			// Ensure old mappings are removed in case the cluster selector is updated.
			if err = removeDesiredMapping(ctx, r.Client, nsoff.Namespace,
				clusterIDMap[virtualNodes.Items[i].Spec.ClusterIdentity.ClusterID]); err != nil {
				returnErr = fmt.Errorf("failed to configure all desired mappings")
			}
		}
	}

	return returnErr
}

func (r *NamespaceOffloadingReconciler) getClusterIDMap(ctx context.Context) (map[string]*virtualkubeletv1alpha1.NamespaceMap, error) {
	// Build the selector to consider only local NamespaceMaps.
	metals := reflection.LocalResourcesLabelSelector()
	selector, err := metav1.LabelSelectorAsSelector(&metals)
	utilruntime.Must(err)

	nms := &virtualkubeletv1alpha1.NamespaceMapList{}
	if err := r.List(ctx, nms, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, fmt.Errorf("failed to retrieve NamespaceMaps: %w", err)
	}

	clusterIDMap := make(map[string]*virtualkubeletv1alpha1.NamespaceMap)
	if len(nms.Items) == 0 {
		klog.Info("No NamespaceMaps are present at the moment in the cluster")
		return clusterIDMap, nil
	}

	for i := range nms.Items {
		clusterIDMap[nms.Items[i].Labels[liqoconst.RemoteClusterID]] = &nms.Items[i]
	}
	return clusterIDMap, nil
}
