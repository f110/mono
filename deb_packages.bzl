load("//build/rules/deb:deb.bzl", "deb_pkg")

def debian_packages():
    deb_pkg(
        name = "debian12_base-files",
        package_name = "base-files",
        sha256 = "6c11268757a89f45c22d7a7a9e1153f1545016987531bf3068765a221e0af86c",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/b/base-files/base-files_12.4+deb12u5_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_base-passwd",
        package_name = "base-passwd",
        sha256 = "908ca1b35125f49125ae56945a72bc11ce0fcec85a8d980d10d83bb3a610f518",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/b/base-passwd/base-passwd_3.6.1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_ca-certificates",
        package_name = "ca-certificates",
        sha256 = "5308b9bd88eebe2a48be3168cb3d87677aaec5da9c63ad0cf561a29b8219115c",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/c/ca-certificates/ca-certificates_20230311_all.deb"],
    )

    deb_pkg(
        name = "debian12_coreutils",
        package_name = "coreutils",
        sha256 = "61038f857e346e8500adf53a2a0a20859f4d3a3b51570cc876b153a2d51a3091",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/c/coreutils/coreutils_9.1-1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_debconf",
        package_name = "debconf",
        sha256 = "74ab14194a3762b2fc717917dcfda42929ab98e3c59295a063344dc551cd7cc8",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/d/debconf/debconf_1.5.82_all.deb"],
    )

    deb_pkg(
        name = "debian12_debianutils",
        package_name = "debianutils",
        sha256 = "55f951359670eb3236c9e2ccd5fac9ccb3db734f5a22aff21589e7a30aee48c9",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/d/debianutils/debianutils_5.7-0.5~deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_dpkg",
        package_name = "dpkg",
        sha256 = "9d97f27d8a8a06dd4800e8e0291337ca02e11cdfd7df09a4566a982a6d9fe4c4",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/d/dpkg/dpkg_1.21.22_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_gcc-12-base",
        package_name = "gcc-12-base",
        sha256 = "1a03df5a57833d65b5bb08cfa19d50e76f29088dc9e64fb934af42d9023a0807",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/gcc-12/gcc-12-base_12.2.0-14_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_git",
        package_name = "git",
        sha256 = "fa5cd07754d9a4f93e2a6f54a5b1fa160230e312121d62c0c609b6701f9b93a3",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/git/git_2.39.2-1.1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_git-man",
        package_name = "git-man",
        sha256 = "34097adaf793f92cc93c8f07059d34766a6a8f2b1d0b1b74b9bb530516402642",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/git/git-man_2.39.2-1.1_all.deb"],
    )

    deb_pkg(
        name = "debian12_libacl1",
        package_name = "libacl1",
        sha256 = "8be9df5795114bfe90e2be3d208ef47a5edd3fc7b3e20d387a597486d444e5e2",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/a/acl/libacl1_2.3.1-3_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libattr1",
        package_name = "libattr1",
        sha256 = "c4945123d66d0503ba42e2fc0585abc76d0838978c6d277b9cc37a4da25d1a34",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/a/attr/libattr1_2.5.1-4_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libbrotli1",
        package_name = "libbrotli1",
        sha256 = "563b4caec1aa5e876bd3355b36e7a38e1484baf5a293b48d1e8bd22db786e4d7",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/b/brotli/libbrotli1_1.0.9-2+b6_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libbz2-1.0",
        package_name = "libbz2-1.0",
        sha256 = "54149da3f44b22d523b26b692033b84503d822cc5122fed606ea69cc83ca5aeb",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/b/bzip2/libbz2-1.0_1.0.8-5+b1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libc6",
        package_name = "libc6",
        sha256 = "0a84833f2f0e6b41e97d8e89917252af83630603e878b8f1202fd9f4ff96cd9f",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/glibc/libc6_2.36-9+deb12u4_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libcom-err2",
        package_name = "libcom-err2",
        sha256 = "8010e4285276bb344c05ae780deae2fffb45e237116c3a78481365c5954125ec",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/e/e2fsprogs/libcom-err2_1.47.0-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libcrypt1",
        package_name = "libcrypt1",
        sha256 = "f5f60a5cdfd4e4eaa9438ade5078a57741a7a78d659fcb0c701204f523e8bd29",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libx/libxcrypt/libcrypt1_4.4.33-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libcurl3-gnutls",
        package_name = "libcurl3-gnutls",
        sha256 = "9751239757dcc218a3cd5a5772070e33d86a8a15506fe5af8a47793d61fa2abc",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/c/curl/libcurl3-gnutls_7.88.1-10+deb12u5_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libdb5.3",
        package_name = "libdb5.3",
        sha256 = "7dc5127b8dd0da80e992ba594954c005ae4359d839a24eb65d0d8129b5235c84",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/d/db5.3/libdb5.3_5.3.28+dfsg2-1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libdebconfclient0",
        package_name = "libdebconfclient0",
        sha256 = "7d2b2b700bae0ba67a13655fabba6a98da3f6ce7dee43d1ee0ac433b7ca1d947",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/c/cdebconf/libdebconfclient0_0.270_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_liberror-perl",
        package_name = "liberror-perl",
        sha256 = "5a466348531b9c38c8e5ccb18c231f27a98b9fdab61b37ea22592553de5d2ced",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libe/liberror-perl/liberror-perl_0.17029-2_all.deb"],
    )

    deb_pkg(
        name = "debian12_libexpat1",
        package_name = "libexpat1",
        sha256 = "fe36a7f35361fc40d0057ef447a7302fd41d51740d51c98fb3870bbed5b96e56",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/e/expat/libexpat1_2.5.0-1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libffi8",
        package_name = "libffi8",
        sha256 = "6d9f6c25c30efccce6d4bceaa48ea86c329a3432abb360a141f76ac223a4c34a",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libf/libffi/libffi8_3.4.4-1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgcc-s1",
        package_name = "libgcc-s1",
        sha256 = "f3d1d48c0599aea85b7f2077a01d285badc42998c1a1e7473935d5cf995c8141",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/gcc-12/libgcc-s1_12.2.0-14_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgdbm-compat4",
        package_name = "libgdbm-compat4",
        sha256 = "4af36a590b68d415a78d9238b932b6a4579f515ec8a8016597498acff5b515a4",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/gdbm/libgdbm-compat4_1.23-3_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgdbm6",
        package_name = "libgdbm6",
        sha256 = "95fe4a1336532450e67bd067892f46eaa484139919ea8d067a9ffcbf5a4bf883",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/gdbm/libgdbm6_1.23-3_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgmp10",
        package_name = "libgmp10",
        sha256 = "187aedef2ed763f425c1e523753b9719677633c7eede660401739e9c893482bd",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/gmp/libgmp10_6.2.1+dfsg1-1.1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgnutls30",
        package_name = "libgnutls30",
        sha256 = "b602f906a8de4488ab75434633524b71b8b12952272d4c3326bd6be8126832c4",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/g/gnutls28/libgnutls30_3.7.9-2+deb12u2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libgssapi-krb5-2",
        package_name = "libgssapi-krb5-2",
        sha256 = "6631304ce4b5b9ba0af3fdebf088a734aed2d28ffad2a03ba79e4fcb2e226dd6",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/k/krb5/libgssapi-krb5-2_1.20.1-2+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libhogweed6",
        package_name = "libhogweed6",
        sha256 = "ed8185c28b2cb519744a5a462dcd720d3b332c9b88a1d0002eac06dc8550cb94",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/n/nettle/libhogweed6_3.8.1-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libidn2-0",
        package_name = "libidn2-0",
        sha256 = "d50716d5824083d667427817d506b45d3f59dc77e1ca52de000f3f62d4918afa",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libi/libidn2/libidn2-0_2.3.3-1+b1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libjansson-dev",
        package_name = "libjansson-dev",
        sha256 = "72ab3de35bd7b0ae26ca71c44a4b72dccab5cc6de8cc08154c5741d0f39b02f7",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/j/jansson/libjansson-dev_2.14-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libjansson4",
        package_name = "libjansson4",
        sha256 = "3fc9742f9f1a37bcb9931df6074b4d1483419ef832ad5349f47323e75fc27864",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/j/jansson/libjansson4_2.14-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libk5crypto3",
        package_name = "libk5crypto3",
        sha256 = "6a91eee690e6ad2207df3a355fc329a58d8e31bf5ca9a9dd4de8f7a1c812ddc5",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/k/krb5/libk5crypto3_1.20.1-2+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libkeyutils1",
        package_name = "libkeyutils1",
        sha256 = "cfac89e6a7a54ff3c6a4f843310e25efeddaa771baeae470bd98bd588c373563",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/k/keyutils/libkeyutils1_1.6.3-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libkrb5-3",
        package_name = "libkrb5-3",
        sha256 = "03ebdf235600f4a8a6d4fbc7080de0a776b1a701f43c4e9697944757591d7809",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/k/krb5/libkrb5-3_1.20.1-2+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libkrb5support0",
        package_name = "libkrb5support0",
        sha256 = "e489a9282c4b765c29d9eda7c4747e1cb58be71161012c3a57e2a8bc63dc0f5a",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/k/krb5/libkrb5support0_1.20.1-2+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libldap-2.5-0",
        package_name = "libldap-2.5-0",
        sha256 = "4b6c30f6554149c594628d945edc6003f0eea8d0cc1341638c0e71375db147ed",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/o/openldap/libldap-2.5-0_2.5.13+dfsg-5_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_liblzma5",
        package_name = "liblzma5",
        sha256 = "d4b7736e58512a2b047f9cb91b71db5a3cf9d3451192fc6da044c77bf51fe869",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/x/xz-utils/liblzma5_5.4.1-0.2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libmd0",
        package_name = "libmd0",
        sha256 = "03539fd30c509e27101d13a56e52eda9062bdf1aefe337c07ab56def25a13eab",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libm/libmd/libmd0_1.0.4-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libnettle8",
        package_name = "libnettle8",
        sha256 = "45922e6e289ffd92f0f92d2bb9159e84236ff202d552a461bf10e5335b3f0261",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/n/nettle/libnettle8_3.8.1-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libnghttp2-14",
        package_name = "libnghttp2-14",
        sha256 = "ee690db978151ae372dcede4bba26c299d985046e6dc708bb907961901b73b6a",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/n/nghttp2/libnghttp2-14_1.52.0-1+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libp11-kit0",
        package_name = "libp11-kit0",
        sha256 = "251330faddbf013f060fcdb41f4b0c037c8a6e89ba7c09b04bfcc4e3f0807b22",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/p/p11-kit/libp11-kit0_0.24.1-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libpcre2-8-0",
        package_name = "libpcre2-8-0",
        sha256 = "030db54f4d76cdfe2bf0e8eb5f9efea0233ab3c7aa942d672c7b63b52dbaf935",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/p/pcre2/libpcre2-8-0_10.42-1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libperl5.36",
        package_name = "libperl5.36",
        sha256 = "4b48b8f0b06c2c667d52117edcef69af6896bcfe69a4f4bde47b89590b83875e",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/p/perl/libperl5.36_5.36.0-7+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libpsl5",
        package_name = "libpsl5",
        sha256 = "4f0d35610204e4e754b057748719744114621f2f6f4202d846c314860a981afb",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libp/libpsl/libpsl5_0.21.2-1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_librtmp1",
        package_name = "librtmp1",
        sha256 = "e1f69020dc2c466e421ec6a58406b643be8b5c382abf0f8989011c1d3df91c87",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/r/rtmpdump/librtmp1_2.4+20151223.gitfa8646d.1-2+b2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libsasl2-2",
        package_name = "libsasl2-2",
        sha256 = "11ee190ad39f8d7af441d2c8347388b9449434c73acc67b4b372445ac4152efa",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/c/cyrus-sasl2/libsasl2-2_2.1.28+dfsg-10_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libsasl2-modules-db",
        package_name = "libsasl2-modules-db",
        sha256 = "3ac4fd6cbe3b3b06e68d24b931bf3eb9385b42f15604a37ed25310e948ca0ee6",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/c/cyrus-sasl2/libsasl2-modules-db_2.1.28+dfsg-10_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libseccomp-dev",
        package_name = "libseccomp-dev",
        sha256 = "ac39081b71c549054ef70d6bc17b4a4bf534c171a86398dbe8e502dbb8565bc6",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libs/libseccomp/libseccomp-dev_2.5.4-1+b3_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libseccomp2",
        package_name = "libseccomp2",
        sha256 = "9e6305a100f5178cc321ee33b96933a6482d11fdc22b42c0e526d6151c0c6f0f",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libs/libseccomp/libseccomp2_2.5.4-1+b3_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libselinux1",
        package_name = "libselinux1",
        sha256 = "2b07f5287b9105f40158b56e4d70cc1652dac56a408f3507b4ab3d061eed425f",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libs/libselinux/libselinux1_3.4-1+b6_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libssh2-1",
        package_name = "libssh2-1",
        sha256 = "d20a3ee34fa84ad8bd381e8be6e9c2c2ea32347cff5e1169c10e978d43f54f24",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libs/libssh2/libssh2-1_1.10.0-3+b1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libssl3",
        package_name = "libssl3",
        sha256 = "6e129c5814812b3516a656ae5b664b9970e2f8823250cd5b98190f21c0de2bca",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/o/openssl/libssl3_3.0.11-1~deb12u2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libtasn1-6",
        package_name = "libtasn1-6",
        sha256 = "eec4dc9d949d2c666b1da3fa762a340e8ba10c3a04d3eed32749a97695c15641",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libt/libtasn1-6/libtasn1-6_4.19.0-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libunistring2",
        package_name = "libunistring2",
        sha256 = "d466bbfe011d764d793c1d9d777cad9c7cf65b938e11598f27408171ad95a951",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libu/libunistring/libunistring2_1.0-2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_libzstd1",
        package_name = "libzstd1",
        sha256 = "6315b5ac38b724a710fb96bf1042019398cb656718b1522279a5185ed39318fa",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/libz/libzstd/libzstd1_1.5.4+dfsg2-5_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_netbase",
        package_name = "netbase",
        sha256 = "29b23c48c0fe6f878e56c5ddc9f65d1c05d729360f3690a593a8c795031cd867",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/n/netbase/netbase_6.4_all.deb"],
    )

    deb_pkg(
        name = "debian12_openssl",
        package_name = "openssl",
        sha256 = "ea063646d4f70d15be5ed52b67b5ac95d68dda823c60d808c7c25439c6d14e4d",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/o/openssl/openssl_3.0.11-1~deb12u2_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_perl",
        package_name = "perl",
        sha256 = "95224197cc1275ee3e625be4522f9d03f8fea3bd7a5d7d8f1f55ab914736b404",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/p/perl/perl_5.36.0-7+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_perl-base",
        package_name = "perl-base",
        sha256 = "b4327c2d8e2ca92402205ac6b5845b3110fa2a1d50925c0e61c39624583a8baf",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/p/perl/perl-base_5.36.0-7+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_perl-modules-5.36",
        package_name = "perl-modules-5.36",
        sha256 = "9b8223674661ead1836ce21966f7e4511a3a943c1b87c02ea92ec17ed2c3f2cf",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/p/perl/perl-modules-5.36_5.36.0-7+deb12u1_all.deb"],
    )

    deb_pkg(
        name = "debian12_tar",
        package_name = "tar",
        sha256 = "24fb92e98c2969171f81a8b589263d705f6b1670f95d121cd74c810d4605acc3",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/t/tar/tar_1.34+dfsg-1.2+deb12u1_amd64.deb"],
    )

    deb_pkg(
        name = "debian12_tzdata",
        package_name = "tzdata",
        sha256 = "0ca0baec1fca55df56039047a631fc1541c5a44c1c4879d553aaa3a70844eb12",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/t/tzdata/tzdata_2024a-0+deb12u1_all.deb"],
    )

    deb_pkg(
        name = "debian12_zlib1g",
        package_name = "zlib1g",
        sha256 = "d7dd1d1411fedf27f5e27650a6eff20ef294077b568f4c8c5e51466dc7c08ce4",
        urls = ["https://snapshot.debian.org/archive/debian/20240510T205852Z/pool/main/z/zlib/zlib1g_1.2.13.dfsg-1_amd64.deb"],
    )

