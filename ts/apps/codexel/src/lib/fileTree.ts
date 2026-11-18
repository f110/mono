// TreeNode is a line of the file tree. The directories are shown before the files.
export type TreeNode = { path: string; name: string; depth: number; dir: boolean }

// visibleNodes returns the nodes of the tree which are not in the collapsed directories. Only the directories in
// expanded are expanded.
export const visibleNodes = (files: string[], expanded: Set<string>): TreeNode[] => {
  type Dir = { dirs: Map<string, Dir>; files: string[] }
  const root: Dir = { dirs: new Map(), files: [] }
  for (const f of files) {
    const parts = f.split('/')
    let d = root
    for (const p of parts.slice(0, -1)) {
      let next = d.dirs.get(p)
      if (!next) {
        next = { dirs: new Map(), files: [] }
        d.dirs.set(p, next)
      }
      d = next
    }
    d.files.push(parts[parts.length - 1])
  }

  const nodes: TreeNode[] = []
  const walk = (d: Dir, prefix: string, depth: number) => {
    for (const name of [...d.dirs.keys()].sort()) {
      const path = prefix + name
      nodes.push({ path, name, depth, dir: true })
      if (expanded.has(path)) {
        walk(d.dirs.get(name)!, path + '/', depth + 1)
      }
    }
    for (const name of [...d.files].sort()) {
      nodes.push({ path: prefix + name, name, depth, dir: false })
    }
  }
  walk(root, '', 0)
  return nodes
}

// fileRevset returns the revset of the revisions which change the file.
export const fileRevset = (path: string) => `files(root-file:"${path.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}")`
