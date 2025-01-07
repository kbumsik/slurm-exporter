# Development

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Development](#development)
  - [Table of Contents](#table-of-contents)
  - [Build](#build)
    - [Build the exporter binary](#build-the-exporter-binary)
    - [Build the container image](#build-the-container-image)

<!-- mdformat-toc end -->

## Build

To build, you need to install build dependencies. To do this, install
[golang](https://go.dev/doc/install) and make.

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
