def rule_on_github(name, repo_name, version, sha256, strip_prefix = "", archive = "tar.gz", type = "release"):
    s = {"sha256": sha256, "strip_prefix": strip_prefix}
    if type == "release":
        s["urls"] = [
            "https://github.com/{r}/releases/download/{v}/{n}-{v}.{e}".format(n = name, r = repo_name, v = version, e = archive),
            # TODO:
            #            "https://mirror.bucket.x.f110.dev/github.com/{r}/releases/download/{v}/{n}-{v}.{e}".format(n = name, r = repo_name, v = version, e = archive),
        ]
    elif type == "tag":
        s["urls"] = [
            "https://github.com/{r}/archive/refs/tags/{v}.{e}".format(r = repo_name, v = version, e = archive),
            # TODO:
            #            "https://mirror.bucket.x.f110.dev/github.com/{r}/archive/refs/tags/{v}.{e}".format(r = repo_name, v = version, e = archive),
        ]
    return s
