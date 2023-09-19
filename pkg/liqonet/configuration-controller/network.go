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

package configurationcontroller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ForgeNetwork creates a ipamv1alpha1.Network resource.
func ForgeNetwork(cfg networkingv1alpha1.Configuration, cidrType LabelCIDRTypeValue, scheme *runtime.Scheme) *ipamv1alpha1.Network {
	network := &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cfg.Name, cidrType),
			Namespace: cfg.Namespace,
			Labels:    ForgeLabel(cidrType),
		},
		Spec: ipamv1alpha1.NetworkSpec{
			CIDR: cfg.Spec.Remote.CIDR.Pod,
		},
	}
	ctrlutil.SetOwnerReference(&cfg, network, scheme)
	return network
}

// CreateOrGetNetwork creates or gets a ipamv1alpha1.Network resource.
func CreateOrGetNetwork(ctx context.Context, cl client.Client, scheme *runtime.Scheme,
	cfg *networkingv1alpha1.Configuration, cidrType LabelCIDRTypeValue) (*ipamv1alpha1.Network, error) {
	ls := ForgeLabelSelector(cidrType)
	ns := cfg.Namespace
	list, err := getters.ListNetworkByLabel(ctx, cl, ns, ls)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 1 {
		return &list.Items[0], nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("multiple networks found with label selector '%s'", ls)
	}
	network := ForgeNetwork(*cfg, cidrType, scheme)

	if err := cl.Create(ctx, network); err != nil {
		return nil, err
	}
	return network, nil
}
