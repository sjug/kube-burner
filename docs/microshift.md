# MicroShift

Kube-burner can run Kubernetes-native workloads against MicroShift and automatically tags indexed results with cluster metadata. The MicroShift path is intentionally detection-based through `go-commons` metadata discovery: there is no required CLI flag and no need to provide distribution keys through `--user-metadata`.

## What is supported

Kube-burner detects cluster distribution at startup through `github.com/cloud-bulldozer/go-commons/v2/ocp-metadata`, stamps a small kube-burner-core metadata subset on indexed documents, gates OpenShift Build waits when the Build API is absent, and provides a MicroShift-specific Prometheus profile at `examples/metrics-profiles/microshift-metrics.yaml`.

The core object lifecycle uses Kubernetes discovery and dynamic clients, so Kubernetes-native workloads run normally when their APIs are present on MicroShift.

## Detection semantics

Kube-burner delegates distribution detection, API-group capability discovery, and MicroShift version discovery to `go-commons` v2.3.4 or newer. The current `go-commons` detection precedence is:

1. `kube-public/microshift-version` ConfigMap exists: MicroShift.
2. `route.openshift.io` exists and `config.openshift.io` does not: MicroShift.
3. `config.openshift.io` exists: OpenShift.
4. Otherwise: Kubernetes.

When MicroShift is detected, kube-burner logs:

```text
🔬 Detected MicroShift cluster
```

The ConfigMap check is first because a plain Kubernetes cluster can install the Route API without becoming MicroShift. Kube-burner keeps a narrow cluster facade over `go-commons` so core code can use the shared API-group capability data for kube-burner behavior such as Build waiter gating without emitting the full OpenShift metadata surface.

## Cluster metadata stamp

The following kube-burner-core metadata keys are applied automatically to `init` and `measure` output on MicroShift:

```yaml
distribution: microshift
microshift: true
k8sVersion: v1.35.3
totalNodes: 1
microshiftVersion: 4.22.0~rc.2
microshiftMajorVersion: "4.22"
```

`microshiftVersion` and `microshiftMajorVersion` are populated only for MicroShift and only when `go-commons` can discover the MicroShift version data. Cluster metadata is applied after `--user-metadata`, so detected cluster values override stale user-provided distribution keys.

On non-MicroShift clusters, kube-burner core still stamps the shared distribution keys. Kubernetes runs receive `distribution: kubernetes`, `microshift: false`, `k8sVersion`, and `totalNodes`. OpenShift runs through kube-burner core receive `distribution: openshift`, `microshift: false`, `k8sVersion`, and `totalNodes`; kube-burner-ocp remains responsible for richer OpenShift-specific metadata.

Kube-burner core does not write the `platform`, `ocpVersion`, or `clusterType` keys for this metadata. Existing kube-burner indices use `platform` for the cloud/provider meaning, such as `AWS`, and kube-burner-ocp remains responsible for rich OpenShift metadata. MicroShift support in kube-burner core intentionally stamps only the distribution-oriented keys listed above.

`JobSummary` documents receive these keys as top-level fields. Prometheus metric and measurement documents keep the existing shape and store them under the nested `metadata` object.

## Metrics profile

Use:

```text
examples/metrics-profiles/microshift-metrics.yaml
```

The profile expects an external Prometheus scraping MicroShift directly. It uses:

- `job="kubelet-microshift"` for kubelet, API server, and embedded etcd client metrics.
- `job="kubelet-microshift-cadvisor"` for cAdvisor container metrics.
- `job="crio"` for CRI-O process and operation metrics.
- `job="node"` for node exporter metrics.
- `job="process"` for named-process-exporter system component metrics.

The MicroShift profile does not use kube-state-metrics, OpenShift `cluster:*` recording rules, `cluster_version`, `kube_node_role`, or standalone `openshift-etcd` scrape metrics. Resource counts come from `apiserver_storage_objects`.

## Workload compatibility

Kubernetes-native workloads should run out of the box when their APIs are available. Good starting points include:

- `examples/workloads/kubelet-density`
- `examples/workloads/service-latency-example`
- `examples/workloads/network-policy`
- `examples/workloads/crd-scale`
- `examples/workloads/api-intensive`
- `examples/workloads/deployment-pvc-move`

Workloads that depend on optional MicroShift add-ons are gated by those add-ons. For example, `udn-density-l3` requires UDN support, and `kubevirt-*` workloads require KubeVirt, which is not present on stock MicroShift.

## What does not work

MicroShift does not expose the OpenShift Build API, so `Build` and `BuildConfig` waits are skipped when `build.openshift.io` is absent.

MicroShift also does not include the OpenShift in-cluster monitoring stack or a standalone external etcd scrape by default. Use the MicroShift metrics profile instead of the OpenShift profiles for direct MicroShift Prometheus scraping.

## Subcommand coverage

`kube-burner init` automatically stamps `JobSummary`, Prometheus metric, and measurement documents.

`kube-burner measure` automatically stamps standalone measurement documents.

`kube-burner index` does not auto-probe the current cluster. It can index historical Prometheus ranges from a different host or context than the original workload run, so automatic probing could stamp misleading metadata. Use `--user-metadata` for `index` and `import` workflows when metadata is needed.

## End-to-end verification

Build kube-burner and confirm the MicroShift context is reachable:

```bash
make build
oc get nodes
```

Create an endpoint file that points at the external Prometheus and references the MicroShift metrics profile. The `init` command reads metric profiles from the endpoint configuration; `-m/--metrics-profile` is used by `index`.

```yaml
- endpoint: http://<prometheus-host>:9091
  indexer:
    type: local
    metricsDirectory: collected-metrics-microshift-smoke
  metrics:
    - examples/metrics-profiles/microshift-metrics.yaml
```

Then run a small workload against the MicroShift context:

```bash
./kube-burner init \
  -c examples/workloads/kubelet-density/kubelet-density.yml \
  -e /tmp/microshift-endpoint.yml \
  --uuid microshift-smoke-$(date +%s)
```

Verify:

- The log contains `🔬 Detected MicroShift cluster` once.
- `jobSummary.json` contains top-level `distribution`, `microshift`, `k8sVersion`, and `totalNodes`.
- On MicroShift, `jobSummary.json` also contains `microshiftVersion` and `microshiftMajorVersion` when the version ConfigMap is populated.
- Scraped metric files contain the same metadata under the nested `metadata` object.
- At least one API, node, container, and named-process metric file is non-empty.
- `APIStorageObjectCount.json` is non-empty, proving `apiserver_storage_objects` is available as the kube-state-metrics replacement.
