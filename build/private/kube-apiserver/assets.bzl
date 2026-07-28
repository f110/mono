# kube-apiserver is distributed as part of the controller-tools envtest bundle
# (which also ships etcd and kubectl). Unlike dl.k8s.io, this bundle provides
# darwin binaries, so it works for local development on macOS.
KUBE_APISERVER_ASSETS = {
    "1.29.4": {
        "linux": {
            "amd64": (
                "https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v1.29.4/envtest-v1.29.4-linux-amd64.tar.gz",
                "e776c2b79d059128f47eb5ccc1f070a16735bf3161ac0be6da4dc4c9b188abd9",
                "controller-tools/envtest",
            ),
        },
        "darwin": {
            "arm64": (
                "https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v1.29.4/envtest-v1.29.4-darwin-arm64.tar.gz",
                "49a8b0ea7b8454880c011452f19a5b3fced6ac2579fe9b0885ac337031c0eb87",
                "controller-tools/envtest",
            ),
            "amd64": (
                "https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v1.29.4/envtest-v1.29.4-darwin-amd64.tar.gz",
                "1254666f9a2dd880250e89f5f390f1e93b60b6fa415de1fba56dbf2f8a30da50",
                "controller-tools/envtest",
            ),
        },
    },
}
