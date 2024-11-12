# Development

To build, you need to install build dependencies. To do this, install
[golang](https://go.dev/doc/install) and make.

## Build

### Build the exporter binary

To build the binary:

```sh
make build
```

### Build the container image

To build the slurm-exporter container image:

```sh
make docker-build docker-push IMG=<registry>/slurm-exporter:<TAG>
```
