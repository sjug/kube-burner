// Copyright 2026 The Kube-burner Authors.
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

package cluster

import (
	"testing"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/v2/ocp-metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestPopulateAPIGroups(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	discovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatal("unexpected discovery type")
	}
	discovery.Resources = []*metav1.APIResourceList{
		{GroupVersion: APIGroupRoute + "/v1"},
		{GroupVersion: APIGroupBuild + "/v1"},
	}

	apiGroups := make(map[string]bool)
	if err := populateAPIGroups(client, apiGroups); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, apiGroup := range []string{APIGroupRoute, APIGroupBuild} {
		if !apiGroups[apiGroup] {
			t.Fatalf("expected API group %q to be present", apiGroup)
		}
	}
	if apiGroups[APIGroupConfig] {
		t.Fatalf("did not expect API group %q to be present", APIGroupConfig)
	}
}

func TestAPIGroupConstantsReExportGoCommonsValues(t *testing.T) {
	tests := map[string]struct {
		got  string
		want string
	}{
		"config": {
			got:  APIGroupConfig,
			want: ocpmetadata.APIGroupOpenShiftConfig,
		},
		"route": {
			got:  APIGroupRoute,
			want: ocpmetadata.APIGroupOpenShiftRoute,
		},
		"build": {
			got:  APIGroupBuild,
			want: ocpmetadata.APIGroupOpenShiftBuild,
		},
		"security": {
			got:  APIGroupSecurity,
			want: ocpmetadata.APIGroupOpenShiftSecurity,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, tc.got)
			}
		})
	}
}

func TestInfoHas(t *testing.T) {
	info := Info{APIGroups: map[string]bool{APIGroupBuild: true}}
	if !info.Has(APIGroupBuild) {
		t.Fatalf("expected Has(%q) to return true", APIGroupBuild)
	}
	if info.Has(APIGroupConfig) {
		t.Fatalf("expected Has(%q) to return false", APIGroupConfig)
	}
}

func TestProbeWithoutRestConfigKeepsCapabilities(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	discovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatal("unexpected discovery type")
	}
	discovery.Resources = []*metav1.APIResourceList{
		{GroupVersion: APIGroupBuild + "/v1"},
	}

	info, err := Probe(client, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Has(APIGroupBuild) {
		t.Fatalf("expected API group %q to be present", APIGroupBuild)
	}
	if info.Distribution != DistributionKubernetes {
		t.Fatalf("expected default distribution %q, got %q", DistributionKubernetes, info.Distribution)
	}
}

func TestApplyClusterMetadataMapsBoundaryFields(t *testing.T) {
	info := Info{APIGroups: map[string]bool{APIGroupBuild: true}}
	applyClusterMetadata(&info, ocpmetadata.ClusterMetadata{
		Distribution:           DistributionMicroShift,
		MicroShift:             true,
		K8SVersion:             "v1.35.3",
		MicroShiftVersion:      "4.22.0~rc.2",
		MicroShiftMajorVersion: "4.22",
		TotalNodes:             1,
		Platform:               "AWS",
		OCPVersion:             "4.20.0",
		ClusterType:            "self-managed",
	})
	if info.Distribution != DistributionMicroShift {
		t.Fatalf("expected distribution %q, got %q", DistributionMicroShift, info.Distribution)
	}
	if !info.MicroShift {
		t.Fatal("expected microshift=true")
	}
	if info.K8sVersion != "v1.35.3" {
		t.Fatalf("expected k8sVersion v1.35.3, got %q", info.K8sVersion)
	}
	if info.MicroShiftVersion != "4.22.0~rc.2" {
		t.Fatalf("expected microshiftVersion 4.22.0~rc.2, got %q", info.MicroShiftVersion)
	}
	if info.MicroShiftMajorVersion != "4.22" {
		t.Fatalf("expected microshiftMajorVersion 4.22, got %q", info.MicroShiftMajorVersion)
	}
	if info.TotalNodes != 1 {
		t.Fatalf("expected totalNodes 1, got %d", info.TotalNodes)
	}
	if !info.Has(APIGroupBuild) {
		t.Fatal("expected existing API group capabilities to be preserved")
	}
}

func TestAsMetadataDoesNotLeakOpenShiftFields(t *testing.T) {
	info := Info{}
	applyClusterMetadata(&info, ocpmetadata.ClusterMetadata{
		Distribution:     DistributionOpenShift,
		K8SVersion:       "v1.34.1",
		TotalNodes:       9,
		Platform:         "AWS",
		ClusterType:      "self-managed",
		OCPVersion:       "4.20.0",
		OCPMajorVersion:  "4.20",
		MasterNodesType:  "m6i.xlarge",
		WorkerNodesType:  "m6i.2xlarge",
		InfraNodesType:   "m6i.large",
		MasterNodesCount: 3,
		WorkerNodesCount: 4,
		InfraNodesCount:  2,
		OtherNodesCount:  1,
		SDNType:          "OVNKubernetes",
		ClusterName:      "example-cluster",
		Region:           "us-east-1",
		Fips:             true,
		Publish:          "External",
		WorkerArch:       "amd64",
		ControlPlaneArch: "amd64",
		Ipsec:            true,
		IpsecMode:        "full",
	})

	metadata := info.AsMetadata()
	expectedKeys := []string{MetadataKeyDistribution, MetadataKeyMicroShift, MetadataKeyK8sVersion, MetadataKeyTotalNodes}
	if len(metadata) != len(expectedKeys) {
		t.Fatalf("expected only %d metadata keys, got %d: %v", len(expectedKeys), len(metadata), metadata)
	}
	for _, key := range expectedKeys {
		if _, ok := metadata[key]; !ok {
			t.Fatalf("expected key %q in metadata", key)
		}
	}
	for _, key := range []string{
		"platform",
		"clusterType",
		"ocpVersion",
		"ocpMajorVersion",
		"masterNodesType",
		"workerNodesType",
		"masterNodesCount",
		"infraNodesType",
		"workerNodesCount",
		"infraNodesCount",
		"otherNodesCount",
		"sdnType",
		"clusterName",
		"region",
		"fips",
		"publish",
		"workerArch",
		"controlPlaneArch",
		"ipsec",
		"ipsecMode",
	} {
		if _, ok := metadata[key]; ok {
			t.Fatalf("did not expect OpenShift field %q to be emitted by kube-burner core", key)
		}
	}
}

func TestApplyClusterMetadataUsesPartialOpenShiftMetadata(t *testing.T) {
	info := Info{Distribution: DistributionKubernetes}
	applyClusterMetadata(&info, ocpmetadata.ClusterMetadata{
		Distribution: DistributionOpenShift,
		K8SVersion:   "v1.34.1",
		TotalNodes:   6,
	})
	if info.Distribution != DistributionOpenShift {
		t.Fatalf("expected distribution %q, got %q", DistributionOpenShift, info.Distribution)
	}
	if info.K8sVersion != "v1.34.1" {
		t.Fatalf("expected k8sVersion v1.34.1, got %q", info.K8sVersion)
	}
	if info.TotalNodes != 6 {
		t.Fatalf("expected totalNodes 6, got %d", info.TotalNodes)
	}
}

func TestHasUsableClusterMetadata(t *testing.T) {
	tests := map[string]struct {
		metadata ocpmetadata.ClusterMetadata
		want     bool
	}{
		"empty": {
			metadata: ocpmetadata.ClusterMetadata{},
			want:     false,
		},
		"missing distribution": {
			metadata: ocpmetadata.ClusterMetadata{
				K8SVersion: "v1.34.1",
				TotalNodes: 3,
			},
			want: false,
		},
		"distribution present": {
			metadata: ocpmetadata.ClusterMetadata{
				Distribution: DistributionKubernetes,
			},
			want: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := hasUsableClusterMetadata(tc.metadata); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestApplyMetadataOverwritesClusterKeys(t *testing.T) {
	info := Info{
		Distribution:           DistributionMicroShift,
		MicroShift:             true,
		K8sVersion:             "v1.35.3",
		MicroShiftVersion:      "4.22.0~rc.2",
		MicroShiftMajorVersion: "4.22",
		TotalNodes:             1,
	}
	metadata := map[string]any{
		MetadataKeyDistribution:           DistributionOpenShift,
		MetadataKeyMicroShift:             false,
		MetadataKeyMicroShiftVersion:      "4.21.0",
		MetadataKeyMicroShiftMajorVersion: "4.21",
		MetadataKeyTotalNodes:             99,
		"platform":                        "AWS",
		"customKey":                       "preserve-me",
	}
	got := info.ApplyMetadata(metadata)
	got["sentinel"] = "same-map"
	if metadata["sentinel"] != "same-map" {
		t.Fatal("expected ApplyMetadata to mutate the existing map instance")
	}
	if metadata[MetadataKeyDistribution] != DistributionMicroShift {
		t.Fatalf("expected distribution to be overwritten, got %v", metadata[MetadataKeyDistribution])
	}
	if metadata[MetadataKeyMicroShift] != true {
		t.Fatalf("expected microshift=true, got %v", metadata[MetadataKeyMicroShift])
	}
	if metadata[MetadataKeyMicroShiftVersion] != "4.22.0~rc.2" {
		t.Fatalf("expected microshiftVersion to be overwritten, got %v", metadata[MetadataKeyMicroShiftVersion])
	}
	if metadata[MetadataKeyMicroShiftMajorVersion] != "4.22" {
		t.Fatalf("expected microshiftMajorVersion to be overwritten, got %v", metadata[MetadataKeyMicroShiftMajorVersion])
	}
	if metadata[MetadataKeyTotalNodes] != 1 {
		t.Fatalf("expected totalNodes to be overwritten, got %v", metadata[MetadataKeyTotalNodes])
	}
	if metadata["platform"] != "AWS" {
		t.Fatalf("expected cloud platform metadata to be preserved, got %v", metadata["platform"])
	}
	if metadata["customKey"] != "preserve-me" {
		t.Fatalf("expected custom user metadata key to be preserved, got %v", metadata["customKey"])
	}
}

func TestAsMetadata(t *testing.T) {
	emptyMetadata := Info{}.AsMetadata()
	if _, ok := emptyMetadata[MetadataKeyMicroShiftVersion]; ok {
		t.Fatal("did not expect microshiftVersion for non-MicroShift cluster")
	}
	if _, ok := emptyMetadata[MetadataKeyMicroShiftMajorVersion]; ok {
		t.Fatal("did not expect microshiftMajorVersion for non-MicroShift cluster")
	}
	if emptyMetadata[MetadataKeyDistribution] != DistributionKubernetes {
		t.Fatalf("expected empty distribution to default to kubernetes, got %v", emptyMetadata[MetadataKeyDistribution])
	}

	openShiftMetadata := Info{Distribution: DistributionOpenShift}.AsMetadata()
	if openShiftMetadata[MetadataKeyDistribution] != DistributionOpenShift {
		t.Fatalf("expected distribution openshift, got %v", openShiftMetadata[MetadataKeyDistribution])
	}
	if _, ok := openShiftMetadata[MetadataKeyMicroShiftVersion]; ok {
		t.Fatal("did not expect microshiftVersion for OpenShift cluster")
	}
	if _, ok := openShiftMetadata[MetadataKeyMicroShiftMajorVersion]; ok {
		t.Fatal("did not expect microshiftMajorVersion for OpenShift cluster")
	}
	if _, ok := openShiftMetadata["openshift"]; ok {
		t.Fatal("did not expect openshift boolean metadata")
	}
	if _, ok := openShiftMetadata["platform"]; ok {
		t.Fatal("did not expect kube-burner core to duplicate kube-burner-ocp platform metadata")
	}

	microShiftWithoutVersionMetadata := Info{
		Distribution: DistributionMicroShift,
		MicroShift:   true,
	}.AsMetadata()
	if _, ok := microShiftWithoutVersionMetadata[MetadataKeyMicroShiftVersion]; ok {
		t.Fatal("did not expect microshiftVersion when MicroShift version is unknown")
	}
	if _, ok := microShiftWithoutVersionMetadata[MetadataKeyMicroShiftMajorVersion]; ok {
		t.Fatal("did not expect microshiftMajorVersion when MicroShift version is unknown")
	}

	info := Info{
		Distribution:           DistributionMicroShift,
		MicroShift:             true,
		K8sVersion:             "v1.35.3",
		MicroShiftVersion:      "4.22.0~rc.2",
		MicroShiftMajorVersion: "4.22",
		TotalNodes:             1,
	}
	metadata := info.AsMetadata()
	expectedKeys := []string{MetadataKeyDistribution, MetadataKeyMicroShift, MetadataKeyMicroShiftVersion, MetadataKeyMicroShiftMajorVersion, MetadataKeyK8sVersion, MetadataKeyTotalNodes}
	if len(metadata) != len(expectedKeys) {
		t.Fatalf("expected only %d metadata keys, got %d: %v", len(expectedKeys), len(metadata), metadata)
	}
	for _, key := range expectedKeys {
		if _, ok := metadata[key]; !ok {
			t.Fatalf("expected key %q in metadata", key)
		}
	}
	if _, ok := metadata["platform"]; ok {
		t.Fatal("did not expect distribution metadata to use platform")
	}
}

func TestApplyMetadataNilInput(t *testing.T) {
	info := Info{Distribution: DistributionKubernetes}
	metadata := info.ApplyMetadata(nil)
	if metadata == nil {
		t.Fatal("expected ApplyMetadata to allocate a map for nil input")
	}
	if metadata[MetadataKeyDistribution] != DistributionKubernetes {
		t.Fatalf("expected distribution kubernetes, got %v", metadata[MetadataKeyDistribution])
	}
}
