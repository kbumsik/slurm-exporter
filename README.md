# slurm-exporter

This project implements a [Prometheus] exporter for [Slurm] metrics.

## Overview

Slurm metrics are collected from Slurm through the [Slurm REST API]. The current
deployment mechanism for this project is through the included [Helm] chart. The
exporter may be built and deployed as a standalone binary if needed.

This project is intended to complement [Kubernetes] and the \[Slurm Operator\].
When configured, the metrics may be used to trigger autoscaling of Slurm nodes
running in [Kubernetes]. As a convenience, the slurm exporter chart is included
as a helm dependency of the slurm helm chart.

## Prerequisites:

Install `kube-prometheus-stack` for metrics collection and observation.
Prometheus is also used as an extenstion API server so custom Slurm metrics may
be used with autoscaling.

```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/kube-prometheus-stack \
  --set installCRDs=true
```

## Configure

Helm is the recommended method to install this project on your Kubernetes
cluster.

In addition to the prerequisites, you should install the both the slurm operator
helm chart and the slurm components helm chart (controller, metrics server, et
cetera). These charts can be found in the [slurm-operator] repository. By
default, the slurm helm chart installs the slurm exporter chart as a dependency.

### Deploy the helm chart

Next, to deploy the helm chart, run the following

```sh
helm install slurm-exporter helm/slurm-exporter
```

To view metrics being collected, run

```sh
kubectl port-forward services/slurm-exporter 8080:8080
```

In another sessions, scrape the `/metrics` endpoint of the exporter:

```sh
curl http://localhost:8080/metrics
```

If `slurm-exporter` is configured correctly and able to communicate to the Slurm
REST API server, Slurm metrics should be returned.

You can also point a browser to this endpoint.

## Restrictions

Currently only a minimal set of metrics are collected. More metrics will be
added in the future.

## License

Copyright (C) SchedMD LLC.

Licensed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0) you
may not use project except in compliance with the license.

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.

<!-- links -->

[helm]: https://helm.sh/
[kubernetes]: https://kubernetes.io/
[prometheus]: https://prometheus.io/
[slurm]: https://slurm.schedmd.com/overview.html
[slurm rest api]: https://slurm.schedmd.com/rest_api.html
[slurm-operator]: https://github.com/SlinkyProject/slurm-operator
