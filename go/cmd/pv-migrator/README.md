pv-migrator
---

This tool will be mirroring two directory. It supports to migrate PersistentVolume to other PersistentVolume.

## How to use

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: grafana
spec:
  template:
    spec:
      initContainers:
        - name: move-data
          image: registry.f110.dev/tools/pv-migrator@sha256:0166fca135a4af078125cd5ef0f2532a0b0247bbc9b71db677e8149417f21210
          args:
            - --source
            - /old-data
            - --destination
            - /new-data
          volumeMounts:
            - name: old-data
              mountPath: /old-data
            - name: ext4-grafana-data
              mountPath: /new-data
```
