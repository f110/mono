package_dependencies = {
    "debian12": {
        "libjansson-dev": ["libc6", "libgcc-s1", "libjansson4"],
        "libseccomp-dev": ["libc6", "libgcc-s1", "libseccomp2"],
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