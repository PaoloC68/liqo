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

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	statuslocal "github.com/liqotech/liqo/pkg/liqoctl/status/local"
	statuspeer "github.com/liqotech/liqo/pkg/liqoctl/status/peer"
)

// TODO change message
const liqoctlStatusLongHelp = `Show the status of Liqo.

The command performs a set of checks to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in a human-readable format.

Examples:
  $ {{ .Executable }} status
or
  $ {{ .Executable }} status --namespace liqo-system
`

// TODO change message
const liqoctlStatusLocalLongHelp = `Show the status of Liqo.

The command performs a set of checks to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in a human-readable format.

Examples:
  $ {{ .Executable }} status
or
  $ {{ .Executable }} status --namespace liqo-system
`

// TODO Change message
const liqoctlStatusPeerLongHelp = `Show the status of Liqo.

The command performs a set of checks to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in a human-readable format.

Examples:
  $ {{ .Executable }} status
or
  $ {{ .Executable }} status --namespace liqo-system
`

// TODO Change message
const liqoctlStatusPeersLongHelp = `Show the status of Liqo.

The command performs a set of checks to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in a human-readable format.

Examples:
  $ {{ .Executable }} status
or
  $ {{ .Executable }} status --namespace liqo-system
`

func newStatusCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := status.Options{Factory: f}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of Liqo",
		Long:  WithTemplate(liqoctlStatusLongHelp),
		Args:  cobra.NoArgs,
	}

	f.AddLiqoNamespaceFlag(cmd.PersistentFlags())
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))

	cmd.PersistentFlags().BoolVar(&options.Verbose, "verbose", false, "Show more informations about the peering")

	cmd.AddCommand(newStatusLocalCommand(ctx, &options))
	cmd.AddCommand(newStatusPeerCommand(ctx, &options))

	return cmd
}

func newStatusLocalCommand(ctx context.Context, statusOptions *status.Options) *cobra.Command {
	options := statuslocal.Options{Options: statusOptions}
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Get information about local cluster",
		Long:  WithTemplate(liqoctlStatusLocalLongHelp),
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.Run(ctx))
		},
	}

	return cmd
}

func newStatusPeerCommand(ctx context.Context, statusOptions *status.Options) *cobra.Command {
	options := statuspeer.Options{Options: statusOptions}
	cmd := &cobra.Command{
		Use:               "peer",
		Aliases:           []string{"peers"},
		Short:             "Get information about a peered cluster",
		Long:              WithTemplate(liqoctlStatusPeerLongHelp),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completion.ForeignClusters(ctx, options.Factory, completion.NoLimit),

		Run: func(cmd *cobra.Command, args []string) {
			options.RemoteClusterNames = args
			output.ExitOnErr(options.Run(ctx))
		},
	}

	return cmd
}
