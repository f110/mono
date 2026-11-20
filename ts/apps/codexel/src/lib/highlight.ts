import type { LanguageFn } from 'highlight.js'
import protobuf from 'highlight.js/lib/languages/protobuf'
import python from 'highlight.js/lib/languages/python'
import { common, createLowlight } from 'lowlight'

// starlark is the grammar of Python with the function calls highlighted. The files of Bazel consist mostly of the
// function calls, so they have little color as Python.
const starlark: LanguageFn = (hljs) => {
  const lang = python(hljs)
  const keywords = (lang.keywords as { keyword: string[] }).keyword
  return {
    ...lang,
    name: 'Starlark',
    aliases: [],
    contains: [
      ...(lang.contains ?? []),
      { scope: 'title.function.invoke', match: new RegExp(`\\b(?!(?:${keywords.join('|')})\\b)[A-Za-z_]\\w*(?=\\s*\\()`), relevance: 0 },
    ],
  }
}

const lowlight = createLowlight({ ...common, protobuf, starlark })

export type Token = { text: string; className?: string }

// The nodes of hast which lowlight returns. Only the fields used here are declared.
type Node = { type: 'text'; value: string } | { type: 'element'; properties?: { className?: string[] }; children: Node[] } | { type: string; children?: Node[] }

const languages: Record<string, string> = {
  go: 'go',
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  py: 'python',
  rs: 'rust',
  rb: 'ruby',
  java: 'java',
  kt: 'kotlin',
  c: 'c',
  h: 'c',
  cc: 'cpp',
  cpp: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  swift: 'swift',
  php: 'php',
  lua: 'lua',
  sh: 'bash',
  bash: 'bash',
  zsh: 'bash',
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  toml: 'ini',
  ini: 'ini',
  md: 'markdown',
  css: 'css',
  scss: 'scss',
  html: 'xml',
  xml: 'xml',
  svg: 'xml',
  sql: 'sql',
  proto: 'protobuf',
  graphql: 'graphql',
  bzl: 'starlark',
  bazel: 'starlark',
}

const fileNames: Record<string, string> = {
  Makefile: 'makefile',
  'BUILD.bazel': 'starlark',
  BUILD: 'starlark',
  'MODULE.bazel': 'starlark',
  WORKSPACE: 'starlark',
}

// languageOf returns the language of highlight.js for the path, or undefined if it is not supported.
export const languageOf = (path: string): string | undefined => {
  const name = path.split('/').pop() ?? ''
  const lang = fileNames[name] ?? languages[name.split('.').pop() ?? '']
  return lang && lowlight.registered(lang) ? lang : undefined
}

// tokenize highlights the line. The lines are highlighted separately, so the constructs over the lines (e.g. the block
// comment) are not recognized.
export const tokenize = (text: string, lang: string | undefined): Token[] => {
  if (!lang || text === '') {
    return [{ text }]
  }
  const tokens: Token[] = []
  const walk = (node: Node, classes: string[]) => {
    if (node.type === 'text' && 'value' in node) {
      tokens.push({ text: node.value, className: classes.length > 0 ? classes.join(' ') : undefined })
      return
    }
    const className = node.type === 'element' && 'properties' in node ? (node.properties?.className ?? []) : []
    for (const child of ('children' in node ? node.children : undefined) ?? []) {
      walk(child, [...classes, ...className])
    }
  }
  walk(lowlight.highlight(lang, text) as unknown as Node, [])
  return tokens
}
