package_dependencies = {
    "debian12": {
        "libjansson-dev": ["libc6", "libgcc-s1", "libjansson4"],
        "libseccomp-dev": ["libc6", "libgcc-s1", "libseccomp2"],
        "base-files": [],
        "base-passwd": ["libc6", "libdebconfclient0", "libgcc-s1", "libpcre2-8-0", "libselinux1"],
        "ca-certificates": ["debconf", "libc6", "libgcc-s1", "libssl3", "openssl"],
        "tzdata": ["debconf"],
        "netbase": [],
        "libc6": ["libc6", "libgcc-s1"],
        "libssl3": ["libc6", "libgcc-s1"],
        "coreutils": ["libacl1", "libattr1", "libc6", "libgcc-s1", "libgmp10", "libpcre2-8-0", "libselinux1"],
        "debianutils": ["libc6", "libgcc-s1"],
        "git": ["dpkg", "git-man", "libacl1", "libbrotli1", "libbz2-1.0", "libc6", "libcom-err2", "libcrypt1", "libcurl3-gnutls", "libdb5.3", "liberror-perl", "libexpat1", "libffi8", "libgcc-s1", "libgdbm-compat4", "libgdbm6", "libgmp10", "libgnutls30", "libgssapi-krb5-2", "libhogweed6", "libidn2-0", "libk5crypto3", "libkeyutils1", "libkrb5-3", "libkrb5support0", "libldap-2.5-0", "liblzma5", "libmd0", "libnettle8", "libnghttp2-14", "libp11-kit0", "libpcre2-8-0", "libperl5.36", "libpsl5", "librtmp1", "libsasl2-2", "libsasl2-modules-db", "libselinux1", "libssh2-1", "libssl3", "libtasn1-6", "libunistring2", "libzstd1", "perl", "perl-base", "perl-modules-5.36", "tar", "zlib1g"],
    }
}

def deb_pkg(distro, *pkgs, excludes = None):
    all = {}
    for x in pkgs:
        if x in excludes:
            continue
        all[x] = None
        for x in package_dependencies[distro][x]:
            if x in excludes:
                continue
            all[x] = None
    return ["@%s_%s//:data" % (distro, k.replace("+", "_")) for k in all]