minio-extra-operator
---

minio-extra-operator is a k8s controller for operating minio.

Officially [minio-operator](https://github.com/minio/minio-operator) can create the instance of minio only.
the instance, which created by minio-operator, is completely empty. it doesn't have any bucket.

This operator can create the bucket.