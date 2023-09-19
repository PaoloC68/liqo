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

import "k8s.io/apimachinery/pkg/labels"

// LabelCIDRType is the label used to target a ipamv1alpha1.Network resource that manages a PodCIDR or an ExternalCIDR.
const LabelCIDRType = "configuration.liqo.io/cidr-type"

// LabelCIDRTypeValue is the value of the LabelCIDRType label.
type LabelCIDRTypeValue string

const (
	// LabelCIDRTypePod is used to target a ipamv1alpha1.Network resource that manages a PodCIDR.
	LabelCIDRTypePod LabelCIDRTypeValue = "pod"
	// LabelCIDRTypeExternal is used to target a ipamv1alpha1.Network resource that manages an ExternalCIDR.
	LabelCIDRTypeExternal LabelCIDRTypeValue = "external"
)

var LabelCIDRTypeValues = []LabelCIDRTypeValue{LabelCIDRTypePod, LabelCIDRTypeExternal}

func ForgeLabel(cidrType LabelCIDRTypeValue) map[string]string {
	return map[string]string{
		LabelCIDRType: string(cidrType),
	}
}

func ForgeLabelSelector(cidrType LabelCIDRTypeValue) labels.Selector {
	return labels.SelectorFromSet(ForgeLabel(cidrType))
}
