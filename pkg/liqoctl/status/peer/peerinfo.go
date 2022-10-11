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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/getters"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
	"github.com/liqotech/liqo/pkg/utils/share"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

// PeerInfoChecker implements the Check interface.
// holds the information about the peered cluster.
type PeerInfoChecker struct {
	options          *Options
	peerInfoSection  output.Section
	collectionErrors []status.CollectionError
}

const (
	peerInfoCheckerName = "Peered Cluster Information"
)

// newPodChecker return a new pod checker.
func newPeerInfoChecker(options *Options) *PeerInfoChecker {
	return &PeerInfoChecker{
		peerInfoSection: output.NewRootSection(),
		options:         options,
	}
}

// Silent implements the Check interface.
func (nc *PeerInfoChecker) Silent() bool {
	return false
}

// Collect implements the collect method of the Checker interface.
// it collects the infos of the peered cluster.
func (pic *PeerInfoChecker) Collect(ctx context.Context) error {
	localClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, pic.options.CRClient, pic.options.LiqoNamespace)
	localClusterName := localClusterIdentity.ClusterName
	if err != nil {
		pic.addCollectionError("LocalClusterName", "", err)
	}

	tunnelEndpoints, err := getters.GetTunnelEndpointsByLabel(ctx, pic.options.CRClient, labels.NewSelector())
	found := false
	if err == nil {
		for i := range tunnelEndpoints.Items {
			te := &tunnelEndpoints.Items[i]
			remoteClusterName := te.Spec.ClusterIdentity.ClusterName
			remoteClusterID := te.Spec.ClusterIdentity.ClusterID
			if !slice.ContainsString(pic.options.RemoteClusterNames, te.Spec.ClusterIdentity.ClusterName) {
				continue
			} else {
				found = true
			}
			foreignCluster, err := getters.GetForeignClusterByClusterID(ctx, pic.options.CRClient, te.Spec.ClusterIdentity.ClusterID)
			if err != nil {
				pic.addCollectionError("ForeignCluster", "unable to collect ForeignCluster", err)
			}

			clusterSection := pic.peerInfoSection.AddSection(te.Spec.ClusterIdentity.ClusterName)

			pic.addPeerSection(clusterSection, foreignCluster)

			pic.addAuthSection(clusterSection, foreignCluster)

			err = pic.addNetworkSection(ctx, clusterSection, foreignCluster, localClusterName)
			if err != nil {
				pic.addCollectionError("Network", "unable to collect Network", err)
			}

			pic.addVpnSection(clusterSection, te)

			err = pic.addResourceSection(ctx, clusterSection, remoteClusterID, localClusterName, remoteClusterName)
			if err != nil {
				pic.addCollectionError("Resources", "unable to collect Resources", err)
			}
		}
	}

	if !found {
		pic.addCollectionError("", "", fmt.Errorf("unable to find peers: \"%s\"", strings.Join(pic.options.RemoteClusterNames, `","`)))
	}

	return nil
}

// addPeerSection adds a section about the peering generic info.
func (pic *PeerInfoChecker) addPeerSection(clusterSection output.Section, foreignCluster *discoveryv1alpha1.ForeignCluster) {
	peerSection := clusterSection.AddSection("Peering")
	incomingStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	peerSection.AddEntry("Incoming", string(incomingStatus))
	outgoingStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	peerSection.AddEntry("Outgoing", string(outgoingStatus))
}

// addAuthSection adds a section about the authentication status.
func (pic *PeerInfoChecker) addAuthSection(clusterSection output.Section, foreignCluster *discoveryv1alpha1.ForeignCluster) {
	authSection := clusterSection.AddSection("Authentication")
	authStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.AuthenticationStatusCondition)
	authSection.AddEntry("Status", string(authStatus))
	if pic.options.Verbose {
		authSection.AddEntry("Auth URL", foreignCluster.Spec.ForeignAuthURL)
	}
}

// addNetworkSection adds a section about the network configuration.
func (pic *PeerInfoChecker) addNetworkSection(ctx context.Context, clusterSection output.Section,
	foreignCluster *discoveryv1alpha1.ForeignCluster, localClusterName string) error {
	networkSection := clusterSection.AddSection("Network")
	networkStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.NetworkStatusCondition)
	networkSection.AddEntry("Networking", string(networkStatus))

	if pic.options.Verbose {
		networkConfigs, err := getters.GetNetworkConfigsByLabel(ctx, pic.options.CRClient,
			foreignCluster.Status.TenantNamespace.Local, labels.NewSelector())
		if err != nil {
			return err
		}

		localNetworkConfigSection := networkSection.AddSection("Local NetworkConfig")
		remoteNetworkConfigSection := networkSection.AddSection("Remote NetworkConfig")
		var selectedSection output.Section
		var remoteSectionMsg, remotePodCIDRMsg, remoteExternalCIDRMsg string
		for i := range networkConfigs.Items {
			nc := &networkConfigs.Items[i]
			if liqonetutils.IsLocalNetworkConfig(nc) {
				selectedSection = localNetworkConfigSection
				remoteSectionMsg = fmt.Sprintf("how %s has been remapped by %s", localClusterName, foreignCluster.Name)
			} else {
				selectedSection = remoteNetworkConfigSection
				remoteSectionMsg = fmt.Sprintf("how %s remapped %s", localClusterName, foreignCluster.Name)
			}

			if nc.Status.PodCIDRNAT == consts.DefaultCIDRValue {
				remotePodCIDRMsg = NotRemappedMsg
			} else {
				remotePodCIDRMsg = nc.Status.PodCIDRNAT
			}
			if nc.Status.ExternalCIDRNAT == consts.DefaultCIDRValue {
				remoteExternalCIDRMsg = NotRemappedMsg
			} else {
				remoteExternalCIDRMsg = nc.Status.ExternalCIDRNAT
			}

			// Collect Original Network Configs
			originalSection := selectedSection.AddSection("Original NetworkConfig")
			originalSection.AddEntry("Pod CIDR", nc.Spec.PodCIDR)
			originalSection.AddEntry("External CIDR", nc.Spec.ExternalCIDR)

			// Collect Remapped Network Configs
			remoteSection := selectedSection.AddSectionWithDetail("Remapped NetworkConfig", remoteSectionMsg)
			remoteSection.AddEntry("Pod CIDR", remotePodCIDRMsg)
			remoteSection.AddEntry("External CIDR", remoteExternalCIDRMsg)
		}
	}
	return nil
}

// addVpnSection adds a section about the VPN configuration.
func (pic *PeerInfoChecker) addVpnSection(clusterSection output.Section, tunnelEndpoint *netv1alpha1.TunnelEndpoint) {
	tunnelEndpointSection := clusterSection.AddSection("VPN Connection")
	vpnEndpointSection := tunnelEndpointSection.AddSection("Gateway")
	vpnEndpointSection.AddEntry("Local", tunnelEndpoint.Status.GatewayIP)
	vpnEndpointSection.AddEntry("Remote", tunnelEndpoint.Status.Connection.PeerConfiguration[wireguard.EndpointIP])
	tunnelEndpointSection.AddEntry("Latency", tunnelEndpoint.Status.Connection.Latency.Value)
	tunnelEndpointSection.AddEntry("Status", fmt.Sprintf("%s - %s",
		string(tunnelEndpoint.Status.Connection.Status),
		tunnelEndpoint.Status.Connection.StatusMessage),
	)
}

// addResourceSection adds a section about the resource usage.
func (pic *PeerInfoChecker) addResourceSection(ctx context.Context, clusterSection output.Section,
	remoteClusterID, localClusterName, remoteClusterName string) error {
	resInTot, err := share.GetIncomingTotal(ctx, pic.options.CRClient, remoteClusterID)
	if err != nil {
		return err
	}
	resOutTot, err := share.GetOutgoingTotal(ctx, pic.options.CRClient, remoteClusterID)
	if err != nil {
		return err
	}
	resInUsed, err := share.GetIncomingUsed(ctx, pic.options.CRClient, remoteClusterID)
	if err != nil {
		return err
	}
	resOutUsed, err := share.GetOutgoingUsed(ctx, pic.options.CRClient, remoteClusterID)
	if err != nil {
		return err
	}

	resourceSection := clusterSection.AddSection("Resources")

	inSection := resourceSection.AddSectionWithDetail("Acquired", fmt.Sprintf("resources offered by %q to %q", remoteClusterName, localClusterName))

	inSection.AddEntry("CPU", fmt.Sprintf("%s/%s", resInUsed.CPUToString(), resInTot.CPUToString()))
	inSection.AddEntry("Memory", fmt.Sprintf("%s/%s", resInUsed.MemoryToString(), resInTot.MemoryToString()))
	inSection.AddEntry("Storage", fmt.Sprintf("%s/%s", resInUsed.StorageToString(), resInTot.StorageToString()))
	inSection.AddEntry("Pods", fmt.Sprintf("%s/%s", resInUsed.Pods.String(), resInTot.Pods.String()))

	outSection := resourceSection.AddSectionWithDetail("Shared", fmt.Sprintf("resources offered by %q to %q", localClusterName, remoteClusterName))
	outSection.AddEntry("CPU", fmt.Sprintf("%s/%s", resOutUsed.CPUToString(), resOutTot.CPUToString()))
	outSection.AddEntry("Memory", fmt.Sprintf("%s/%s", resOutUsed.MemoryToString(), resOutTot.MemoryToString()))
	outSection.AddEntry("Storage", fmt.Sprintf("%s/%s", resOutUsed.StorageToString(), resOutTot.StorageToString()))
	outSection.AddEntry("Pods", fmt.Sprintf("%s/%s", resOutUsed.Pods.String(), resOutTot.Pods.String()))

	return nil
}

// GetTitle implements the getTitle method of the Checker interface.
// it returns the title of the checker.
func (pic *PeerInfoChecker) GetTitle() string {
	return peerInfoCheckerName
}

// Format implements the format method of the Checker interface.
// it outputs the infos about the local cluster in a string ready to be
// printed out.
func (pic *PeerInfoChecker) Format() (string, error) {
	text := ""
	var err error
	if len(pic.collectionErrors) == 0 {
		text, err = pic.peerInfoSection.SprintForBox(pic.options.Printer)
	} else {
		for _, cerr := range pic.collectionErrors {
			text += pic.options.Printer.Error.Sprintfln(pic.options.Printer.Paragraph.Sprintf("%s\t%s\t%s",
				cerr.AppName,
				cerr.AppType,
				cerr.Err))
		}
		text = strings.TrimRight(text, "\n")
	}
	return text, err
}

// HasSucceeded return true if no errors have been kept.
func (pic *PeerInfoChecker) HasSucceeded() bool {
	return len(pic.collectionErrors) == 0
}

// addCollectionError adds a collection error. A collection error is an error that happens while
// collecting the status of a Liqo component.
func (pic *PeerInfoChecker) addCollectionError(peerInfoType, peerInfoName string, err error) {
	pic.collectionErrors = append(pic.collectionErrors, status.NewCollectionError(peerInfoType, peerInfoName, err))
}
