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

// Package fields contain field selectors used throughout the liqo code in order to get
// k8s resources.
package fields

import (
	"k8s.io/apimachinery/pkg/fields"
)

// NameAndNamespaceFieldSelector returns a field selector to match a resource by name and namespace.
func NameAndNamespaceFieldSelector(name, namespace string) fields.Selector {
	fs := fields.Set{}
	fs["metadata.name"] = name
	fs["metadata.namespace"] = namespace
	return fields.SelectorFromSet(fs)
}
