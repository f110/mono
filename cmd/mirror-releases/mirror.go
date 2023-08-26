package main

import (
	"fmt"
)

func init() {
	mirrorBazel("releases.bazel.build",
		"6.3.2", "6.3.1", "6.2.1",
		"5.3.0",
	)
}

func mirrorBazel(pathPrefix string, vers ...string) {
	var osArch = []string{"darwin-arm64", "linux-x86_64"}

	for _, v := range vers {
		for _, platform := range osArch {
			mirrorTargets = append(mirrorTargets, &mirrorAsset{
				Path:              pathPrefix,
				OriginURL:         fmt.Sprintf("https://releases.bazel.build/%s/release/bazel-%s-%s", v, v, platform),
				ChecksumURL:       fmt.Sprintf("https://releases.bazel.build/%s/release/bazel-%s-%s.sha256", v, v, platform),
				ChecksumAlgorithm: SHA256Checksum,
				SignatureURL:      fmt.Sprintf("https://releases.bazel.build/%s/release/bazel-%s-%s.sig", v, v, platform),
				PublicKeyURL:      "https://bazel.build/bazel-release.pub.gpg",
			})
		}
	}
}
