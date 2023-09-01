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

package virtualnode

import (
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

// Selector is a runtime representation of v1.NodeSelector that
// only reports parse errors when no terms match.
type Selector struct {
	terms []selectorTerm
}

type selectorTerm struct {
	matchLabels labels.Selector
	matchFields fields.Selector
	parseErrs   []error
}

var validSelectorOperators = []string{
	string(corev1.NodeSelectorOpIn),
	string(corev1.NodeSelectorOpNotIn),
	string(corev1.NodeSelectorOpExists),
	string(corev1.NodeSelectorOpDoesNotExist),
	string(corev1.NodeSelectorOpGt),
	string(corev1.NodeSelectorOpLt),
}

var validFieldSelectorOperators = []string{
	string(corev1.NodeSelectorOpIn),
	string(corev1.NodeSelectorOpNotIn),
}

// MatchSelectorTerms checks whether the virtualnode labels and fields match node selector terms;
// nil or empty term matches no objects.
func MatchSelectorTerms(virtualnode *virtualkubeletv1alpha1.VirtualNode,
	selector *corev1.NodeSelector) (bool, error) {
	// Shortcircuit the matching logic, to always return a positive outcome in case no selector is specified.
	if len(selector.NodeSelectorTerms) == 0 {
		return true, nil
	}
	if virtualnode == nil {
		return false, nil
	}
	return NewSelector(selector).Match(virtualnode)
}

// NewSelector creates a VirtualNodeSelector that only reports parse
// errors when no terms match.
func NewSelector(ns *corev1.NodeSelector, opts ...field.PathOption) *Selector {
	p := field.ToPath(opts...)
	parsedTerms := make([]selectorTerm, 0, len(ns.NodeSelectorTerms))
	path := p.Child("nodeSelectorTerms")
	for i := range ns.NodeSelectorTerms {
		// nil or empty term selects no objects
		if isEmptyVirtualNodeSelectorTerm(&ns.NodeSelectorTerms[i]) {
			continue
		}
		p := path.Index(i)
		parsedTerms = append(parsedTerms, newVirtualNodeSelectorTerm(&ns.NodeSelectorTerms[i], p))
	}
	return &Selector{
		terms: parsedTerms,
	}
}

// isEmptyVirtualNodeSelectorTerm checks whether the term is empty.
func isEmptyVirtualNodeSelectorTerm(term *corev1.NodeSelectorTerm) bool {
	return len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0
}

func newVirtualNodeSelectorTerm(term *corev1.NodeSelectorTerm, path *field.Path) selectorTerm {
	var parsedTerm selectorTerm
	var errs []error
	if len(term.MatchExpressions) != 0 {
		p := path.Child("matchExpressions")
		parsedTerm.matchLabels, errs = nodeSelectorRequirementsAsSelector(term.MatchExpressions, p)
		if errs != nil {
			parsedTerm.parseErrs = append(parsedTerm.parseErrs, errs...)
		}
	}
	if len(term.MatchFields) != 0 {
		p := path.Child("matchFields")
		parsedTerm.matchFields, errs = virtualNodeSelectorRequirementsAsFieldSelector(term.MatchFields, p)
		if errs != nil {
			parsedTerm.parseErrs = append(parsedTerm.parseErrs, errs...)
		}
	}
	return parsedTerm
}

// virtualNodeSelectorRequirementsAsFieldSelector converts the []NodeSelectorRequirement core type into a struct that implements
// fields.Selector.
func virtualNodeSelectorRequirementsAsFieldSelector(nsr []corev1.NodeSelectorRequirement, path *field.Path) (fields.Selector, []error) {
	if len(nsr) == 0 {
		return fields.Nothing(), nil
	}
	var errs []error

	var selectors []fields.Selector
	for i, expr := range nsr {
		p := path.Index(i)
		switch expr.Operator {
		case corev1.NodeSelectorOpIn:
			if len(expr.Values) != 1 {
				errs = append(errs, field.Invalid(p.Child("values"), expr.Values, "must have one element"))
			} else {
				selectors = append(selectors, fields.OneTermEqualSelector(expr.Key, expr.Values[0]))
			}

		case corev1.NodeSelectorOpNotIn:
			if len(expr.Values) != 1 {
				errs = append(errs, field.Invalid(p.Child("values"), expr.Values, "must have one element"))
			} else {
				selectors = append(selectors, fields.OneTermNotEqualSelector(expr.Key, expr.Values[0]))
			}

		default:
			errs = append(errs, field.NotSupported(p.Child("operator"), expr.Operator, validFieldSelectorOperators))
		}
	}

	if len(errs) != 0 {
		return nil, errs
	}
	return fields.AndSelectors(selectors...), nil
}

// nodeSelectorRequirementsAsSelector converts the []NodeSelectorRequirement api type into a struct that implements
// labels.Selector.
func nodeSelectorRequirementsAsSelector(nsm []corev1.NodeSelectorRequirement, path *field.Path) (labels.Selector, []error) {
	if len(nsm) == 0 {
		return labels.Nothing(), nil
	}
	var errs []error
	selector := labels.NewSelector()
	for i, expr := range nsm {
		p := path.Index(i)
		var op selection.Operator
		switch expr.Operator {
		case corev1.NodeSelectorOpIn:
			op = selection.In
		case corev1.NodeSelectorOpNotIn:
			op = selection.NotIn
		case corev1.NodeSelectorOpExists:
			op = selection.Exists
		case corev1.NodeSelectorOpDoesNotExist:
			op = selection.DoesNotExist
		case corev1.NodeSelectorOpGt:
			op = selection.GreaterThan
		case corev1.NodeSelectorOpLt:
			op = selection.LessThan
		default:
			errs = append(errs, field.NotSupported(p.Child("operator"), expr.Operator, validSelectorOperators))
			continue
		}
		r, err := labels.NewRequirement(expr.Key, op, expr.Values, field.WithPath(p))
		if err != nil {
			errs = append(errs, err)
		} else {
			selector = selector.Add(*r)
		}
	}
	if len(errs) != 0 {
		return nil, errs
	}
	return selector, nil
}

// Match checks whether the virtualnode labels and fields match the selector terms;
// nil or empty term matches no objects.
// Parse errors are only returned if no terms matched.
func (vns *Selector) Match(virtualnode *virtualkubeletv1alpha1.VirtualNode) (bool, error) {
	if virtualnode == nil {
		return false, nil
	}
	nodeLabels := labels.Set(virtualnode.Labels)
	nodeFields := extractVirtualNodeFields(virtualnode)

	var errs []error
	for _, term := range vns.terms {
		match, tErrs := term.match(nodeLabels, nodeFields)
		if len(tErrs) > 0 {
			errs = append(errs, tErrs...)
			continue
		}
		if match {
			// DEBUG: remove before merge
			pterm.FgGreen.Printfln("VirtualNode %s matched", virtualnode.Name)
			return true, nil
		}
		//Debug: remove before merge
		pterm.FgRed.Printfln("VirtualNode %s did not match", virtualnode.Name)
	}
	return false, errors.Flatten(errors.NewAggregate(errs))
}

func extractVirtualNodeFields(virtualnode *virtualkubeletv1alpha1.VirtualNode) fields.Set {
	f := make(fields.Set)
	if len(virtualnode.Name) > 0 {
		f["metadata.name"] = virtualnode.Name
	}
	return f
}

func (t *selectorTerm) match(nodeLabels labels.Set, nodeFields fields.Set) (bool, []error) {
	if t.parseErrs != nil {
		return false, t.parseErrs
	}
	if t.matchLabels != nil && !t.matchLabels.Matches(nodeLabels) {
		return false, nil
	}
	if t.matchFields != nil && len(nodeFields) > 0 && !t.matchFields.Matches(nodeFields) {
		return false, nil
	}
	return true, nil
}
