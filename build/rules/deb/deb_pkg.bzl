package_dependencies = {
    "jammy": {
        "libjansson-dev": ["libjansson4"],
    }
}

def deb_pkg(distro, *pkgs):
    all = {}
    for x in pkgs:
        all[x] = None
        for x in package_dependencies[distro][x]:
            all[x] = None
    return ["@%s_%s//:data" % (distro, k) for k in all]
