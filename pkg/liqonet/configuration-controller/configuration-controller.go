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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// ConfigurationReconciler manage Configuration lifecycle.
type ConfigurationReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
}

// NewConfigurationReconciler returns a new ConfigurationReconciler.
func NewConfigurationReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configuration,verbs=get;list;watch;update

// Reconcile manage NamespaceMaps associated with the virtual-node.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	configuration := &networkingv1alpha1.Configuration{}
	if err := r.Get(ctx, req.NamespacedName, configuration); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no a configuration called '%s' in '%s'", req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf(" %w --> Unable to get the configuration '%s'", err, req.Name)
	}

	requeue, err := r.RemapConfiguration(ctx, configuration)
	if requeue || err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.UpdateConfigurationStatus(ctx, configuration)
}

// RemapConfiguration remap the configuration using ipamv1alpha1.Network.
func (r *ConfigurationReconciler) RemapConfiguration(ctx context.Context, cfg *networkingv1alpha1.Configuration) (requeue bool, err error) {
	var cidrRemapped networkingv1alpha1.CIDR
	for _, cidrType := range LabelCIDRTypeValues {
		switch cidrType {
		case LabelCIDRTypePod:
			cidrRemapped = cfg.Status.Remote.CIDR.Pod
		case LabelCIDRTypeExternal:
			cidrRemapped = cfg.Status.Remote.CIDR.External
		}
		if cidrRemapped != "" {
			continue
		}
		network, err := CreateOrGetNetwork(ctx, r.Client, r.Scheme, cfg, cidrType)
		if err != nil {
			return true, fmt.Errorf(" %w --> Unable to create or get the network '%s'", err, network.Name)
		}
		if network.Status.CIDR == "" {
			return true, nil
		}
		var cidrNew, cidrOld networkingv1alpha1.CIDR
		cidrNew = network.Status.CIDR
		switch cidrType {
		case LabelCIDRTypePod:
			cidrOld = cfg.Status.Remote.CIDR.Pod
			cfg.Status.Remote.CIDR.Pod = network.Status.CIDR
		case LabelCIDRTypeExternal:
			cidrOld = cfg.Status.Remote.CIDR.External
			cfg.Status.Remote.CIDR.External = network.Status.CIDR
		}
		klog.Infof("Configuration %s/%s %s CIDR: %s -> %s", cfg.Name, cfg.Namespace, cidrType, cidrOld, cidrNew)
	}
	return false, nil
}

// UpdateConfigurationStatus update the status of the configuration.
func (r *ConfigurationReconciler) UpdateConfigurationStatus(ctx context.Context, cfg *networkingv1alpha1.Configuration) error {
	if err := r.Status().Update(ctx, cfg); err != nil {
		return fmt.Errorf(" %w --> Unable to update the configuration '%s'", err, cfg.Name)
	}
	return nil
}

// SetupWithManager register the ConfigurationReconciler to the manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Configuration{}).Owns(&ipamv1alpha1.Network{}).
		Complete(r)
}
