#!/bin/sh

CONTAINER=unifibackup

mkdir -p /opt/cni
ln -s /mnt/data/podman/cni /opt/cni/bin

if podman container exists ${CONTAINER}; then
    podman start ${CONTAINER}
else
    podman run -i -d --rm \
        --name ${CONTAINER} \
        -v /mnt/data/unifibackup:/etc/unifibackup \
        -v /mnt/data/unifi-os/unifi/data:/unifi \
        quay.io/f110/unifibackup:latest \
        --dir=/unifi/backup/autobackup \
        --bucket=unificontroller-backup \
        --credential=/etc/unifibackup/unifibackup-credential.json \
        --path-prefix=udmp
fi