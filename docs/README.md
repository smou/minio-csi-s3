# Helm Chart for Minio/AIStor S3 CSI Driver

## Prerequisites

- Kubernetes 1.33+
- Helm v3.2.0+

## Installation

[Helm](https://helm.sh) must be installed to use the charts.  Please refer to
Helm's [documentation](https://helm.sh/docs) to get started.

### Add Helm repository

`helm repo add minio-csi-s3 https://smou.github.io/k8s-csi-s3`

If you had already added this repo earlier, run `helm repo update` to retrieve
the latest versions of the packages.  You can then run `helm search repo
minio-csi-s3` to see the charts.

### Install the chart

To install the minio-csi-s3 chart (Pattern: chartname repo-alias/chartname):

`helm install my-minio-csi-s3 minio-csi-s3/minio-csi-s3`

### Uninstallation

To uninstall/delete `my-minio-csi-s3` deployment:

`helm uninstall my-minio-csi-s3`

## Configuration
The following table lists the configurable parameter of the minio-csi-s3 chart and the default values.

| Parameter           | Description                                           | Default           |
|---------------------|-------------------------------------------------------|-------------------|
| version             | Application version which can be overwritten          | Chart.AppVersion  |
| verbose             | Level of verbosity of the container                   | 0                 |
| nameOverride        | Name of the resource bundle to overwrite the default  | minio-csi-s3      |
| **S3**              | | |
| s3.endpoint         | FQDN to the minio/aistor instance (e.g. http://localhost:9000) | - |
| s3.region           | Takes no effect for minio/aistor | us-east-1 |
| s3.bucketPrefix     | String which will be prefixed to the volumnename      | - |
| **Namespace**       | | |
| namespace.create    | Boolean to define if the space should be created or not | false |
| namespace.name      | Name of target namespace if it will be differs from Release namespace | Release.Namespace |