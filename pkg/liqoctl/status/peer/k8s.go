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

package statuspeer

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/liqotech/liqo/pkg/liqoctl/status"
)

// k8sStatusCollector knows how to interact with k8s cluster.
type k8sStatusCollector struct {
	checkers []status.Checker
	options  *Options
}

// newK8sStatusPeerCollector returns a new k8sStatusCollector.
func newK8sStatusPeerCollector(options *Options) *k8sStatusCollector {
	return &k8sStatusCollector{
		options: options,
		checkers: []status.Checker{
			status.NewNamespaceChecker(options.Options, true),
			newPeerInfoChecker(options),
		},
	}
}

// collectStatusPeer collects the status of each Checker that belongs to the collector.
func (k *k8sStatusCollector) collectStatusPeer(ctx context.Context) error {
	for i, checker := range k.checkers {
		if err := checker.Collect(ctx); err != nil {
			return err
		}

		text, err := checker.Format()
		if !checker.Silent() || err != nil {
			k.options.Printer.BoxSetTitle(checker.GetTitle())
			k.options.Printer.BoxPrintln(text)
		}

		// Errors are printed before returning the error.
		if err != nil {
			return err
		}

		// Insert a new line between each checker.
		if i != len(k.checkers)-1 && !checker.Silent() {
			pterm.Println()
		}
	}
	return nil
}
