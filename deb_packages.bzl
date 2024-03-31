load("//build/rules/deb:deb.bzl", "deb_pkg")

def debian_packages():
    deb_pkg(
        name = "debian12_gcc-12-base",
        package_name = "gcc-12-base",
        sha256 = "1a03df5a57833d65b5bb08cfa19d50e76f29088dc9e64fb934af42d9023a0807",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/g/gcc-12/gcc-12-base_12.2.0-14_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libc6",
        package_name = "libc6",
        sha256 = "0a84833f2f0e6b41e97d8e89917252af83630603e878b8f1202fd9f4ff96cd9f",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/g/glibc/libc6_2.36-9+deb12u4_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgcc-s1",
        package_name = "libgcc-s1",
        sha256 = "f3d1d48c0599aea85b7f2077a01d285badc42998c1a1e7473935d5cf995c8141",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/g/gcc-12/libgcc-s1_12.2.0-14_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libjansson-dev",
        package_name = "libjansson-dev",
        sha256 = "72ab3de35bd7b0ae26ca71c44a4b72dccab5cc6de8cc08154c5741d0f39b02f7",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/j/jansson/libjansson-dev_2.14-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libjansson4",
        package_name = "libjansson4",
        sha256 = "3fc9742f9f1a37bcb9931df6074b4d1483419ef832ad5349f47323e75fc27864",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/j/jansson/libjansson4_2.14-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libseccomp-dev",
        package_name = "libseccomp-dev",
        sha256 = "ac39081b71c549054ef70d6bc17b4a4bf534c171a86398dbe8e502dbb8565bc6",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/libs/libseccomp/libseccomp-dev_2.5.4-1+b3_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libseccomp2",
        package_name = "libseccomp2",
        sha256 = "9e6305a100f5178cc321ee33b96933a6482d11fdc22b42c0e526d6151c0c6f0f",
        urls = ["https://snapshot.debian.org/archive/debian/20240327T025446Z/pool/main/libs/libseccomp/libseccomp2_2.5.4-1+b3_amd64.deb"],
    )

