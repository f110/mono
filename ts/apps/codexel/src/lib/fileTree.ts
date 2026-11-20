// TreeNode is a line of the file tree. The directories are shown before the files. expanded is true for the expanded
// directories.
export type TreeNode = { path: string; name: string; depth: number; dir: boolean; expanded: boolean }

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
      const open = expanded.has(path)
      nodes.push({ path, name, depth, dir: true, expanded: open })
      if (open) {
        walk(d.dirs.get(name)!, path + '/', depth + 1)
      }
    }
    for (const name of [...d.files].sort()) {
      nodes.push({ path: prefix + name, name, depth, dir: false, expanded: false })
    }
  }
  walk(root, '', 0)
  return nodes
}

// fileRevset returns the revset of the revisions which change the file.
export const fileRevset = (path: string) => `files(root-file:"${path.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}")`

// parentDirs returns the directories which contain the file from the top.
export const parentDirs = (path: string): string[] => {
  const parts = path.split('/')
  return parts.slice(0, -1).map((_, i) => parts.slice(0, i + 1).join('/'))
}

// FileRevision is the revision shown in the file browser.
export type FileRevision = { commitId: string; changeId: string }

// FileLocation is the position in the file browser which is kept in the state of the history.
export type FileLocation = { repoPath: string; revision: FileRevision; file: string; scrollTop: number }

const fileLocationKey = 'codexelFile'

// fileLocation returns the position in the file browser of the repository from the state of the history.
export const fileLocation = (state: unknown, repoPath: string): FileLocation | undefined => {
  const l = (state as Record<string, unknown> | undefined)?.[fileLocationKey] as FileLocation | undefined
  return l?.repoPath === repoPath ? l : undefined
}

// withFileLocation returns the state of the history which has the position in the file browser.
export const withFileLocation = (state: object, location: FileLocation): object => ({ ...state, [fileLocationKey]: location })
