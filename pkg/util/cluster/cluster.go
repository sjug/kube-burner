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
	"fmt"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/v2/ocp-metadata"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	APIGroupConfig   = ocpmetadata.APIGroupOpenShiftConfig
	APIGroupRoute    = ocpmetadata.APIGroupOpenShiftRoute
	APIGroupBuild    = ocpmetadata.APIGroupOpenShiftBuild
	APIGroupSecurity = ocpmetadata.APIGroupOpenShiftSecurity
)

const (
	DistributionKubernetes = ocpmetadata.DistributionKubernetes
	DistributionOpenShift  = ocpmetadata.DistributionOpenShift
	DistributionMicroShift = ocpmetadata.DistributionMicroShift
)

const (
	MetadataKeyDistribution           = "distribution"
	MetadataKeyMicroShift             = "microshift"
	MetadataKeyMicroShiftVersion      = "microshiftVersion"
	MetadataKeyMicroShiftMajorVersion = "microshiftMajorVersion"
	MetadataKeyK8sVersion             = "k8sVersion"
	MetadataKeyTotalNodes             = "totalNodes"
)

// Info contains the cluster facts kube-burner core consumes. go-commons is the
// source for distribution metadata, but this type deliberately keeps a narrower
// API than ocpmetadata.ClusterMetadata so kube-burner-ocp remains responsible
// for rich OpenShift metadata.
type Info struct {
	Distribution           string
	MicroShift             bool
	MicroShiftVersion      string
	MicroShiftMajorVersion string
	K8sVersion             string
	TotalNodes             int
	APIGroups              map[string]bool
}

// Has reports whether the probed cluster advertised the given API group.
func (i Info) Has(group string) bool {
	return i.APIGroups[group]
}

// AsMetadata returns the cluster metadata keys kube-burner stamps onto indexed
// documents. The key set stays intentionally small to avoid duplicating
// kube-burner-ocp's richer OpenShift metadata flow in kube-burner core.
func (i Info) AsMetadata() map[string]any {
	distribution := i.Distribution
	if distribution == "" {
		distribution = DistributionKubernetes
	}
	metadata := map[string]any{
		MetadataKeyDistribution: distribution,
		MetadataKeyMicroShift:   i.MicroShift,
		MetadataKeyK8sVersion:   i.K8sVersion,
		MetadataKeyTotalNodes:   i.TotalNodes,
	}
	if i.MicroShiftVersion != "" {
		metadata[MetadataKeyMicroShiftVersion] = i.MicroShiftVersion
	}
	if i.MicroShiftMajorVersion != "" {
		metadata[MetadataKeyMicroShiftMajorVersion] = i.MicroShiftMajorVersion
	}
	return metadata
}

// ApplyMetadata stamps cluster metadata into m, overwriting stale user-provided
// cluster keys. A non-nil map is always returned.
func (i Info) ApplyMetadata(m map[string]any) map[string]any {
	if m == nil {
		m = make(map[string]any)
	}
	for k, v := range i.AsMetadata() {
		m[k] = v
	}
	return m
}

// Probe collects kube-burner API capabilities and shared cluster metadata.
// go-commons is the normal source for both capabilities and metadata. Missing
// rest config skips go-commons metadata and falls back to local capability
// discovery. go-commons errors use partial metadata when useful; empty metadata
// errors are returned so callers avoid stamping wrong data.
func Probe(client kubernetes.Interface, restConfig *rest.Config) (Info, error) {
	info := Info{
		Distribution: DistributionKubernetes,
		APIGroups:    make(map[string]bool),
	}
	if restConfig == nil {
		log.Warn("Skipping go-commons cluster metadata probe: rest config is nil")
		if err := populateAPIGroups(client, info.APIGroups); err != nil {
			log.Warnf("Failed to discover API groups: %v", err)
		}
		return info, nil
	}
	metadataAgent, err := ocpmetadata.NewMetadata(restConfig)
	if err != nil {
		if discoverErr := populateAPIGroups(client, info.APIGroups); discoverErr != nil {
			log.Warnf("Failed to discover API groups after go-commons metadata agent creation failed: %v", discoverErr)
		}
		return info, fmt.Errorf("creating go-commons metadata agent: %w", err)
	}

	clusterInfo, err := metadataAgent.GetClusterInfo()
	for group := range clusterInfo.Capabilities.APIGroups {
		info.APIGroups[group] = true
	}
	if err != nil {
		if len(info.APIGroups) == 0 {
			if discoverErr := populateAPIGroups(client, info.APIGroups); discoverErr != nil {
				log.Warnf("Failed to discover API groups after go-commons GetClusterInfo failed: %v", discoverErr)
			}
		}
		if !hasUsableClusterMetadata(clusterInfo.Metadata) {
			return info, fmt.Errorf("getting cluster info from go-commons: %w", err)
		}
		log.Warnf("Using partial cluster metadata from go-commons after error: %v", err)
	}
	applyClusterMetadata(&info, clusterInfo.Metadata)
	logDetectedDistribution(info.Distribution)
	return info, nil
}

func applyClusterMetadata(info *Info, clusterMetadata ocpmetadata.ClusterMetadata) {
	if clusterMetadata.Distribution != "" {
		info.Distribution = clusterMetadata.Distribution
	}
	info.MicroShift = clusterMetadata.MicroShift
	info.MicroShiftVersion = clusterMetadata.MicroShiftVersion
	info.MicroShiftMajorVersion = clusterMetadata.MicroShiftMajorVersion
	info.K8sVersion = clusterMetadata.K8SVersion
	info.TotalNodes = clusterMetadata.TotalNodes
}

func hasUsableClusterMetadata(clusterMetadata ocpmetadata.ClusterMetadata) bool {
	return clusterMetadata.Distribution != ""
}

func populateAPIGroups(client kubernetes.Interface, apiGroups map[string]bool) error {
	groups, err := client.Discovery().ServerGroups()
	if err != nil {
		return fmt.Errorf("discovering server API groups: %w", err)
	}
	for _, group := range groups.Groups {
		apiGroups[group.Name] = true
	}
	return nil
}

func logDetectedDistribution(distribution string) {
	switch distribution {
	case DistributionMicroShift:
		log.Infof("🔬 Detected MicroShift cluster")
	case DistributionOpenShift:
		log.Infof("🔬 Detected OpenShift cluster")
	default:
		log.Infof("🔬 Detected Kubernetes cluster")
	}
}
