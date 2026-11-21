import { Fragment, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { Box, Button, IconButton, Stack, Tooltip, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import { TranscriptEntryKind, type TranscriptEntry } from '../connect/codexel_pb.js'
import { Markdown } from './Markdown.tsx'

type RecordKind = 'user' | 'assistant' | 'tool'

// Rec is a row of the ledger. The tool row has the call and the result. end is undefined while the tool is running.
type Rec = {
  key: number
  kind: RecordKind
  entry: TranscriptEntry
  result?: TranscriptEntry
  start: number
  end?: number
}

type Item = { type: 'record'; rec: Rec } | { type: 'calls'; key: number; recs: Rec[] }

type Turn = {
  index: number
  prompt?: Rec
  items: Item[]
  start: number
  end: number
}

const kindLabels: Record<RecordKind, string> = { user: 'User', assistant: 'Assistant', tool: 'Tool' }
const kindColors: Record<RecordKind, string> = { user: 'info.main', assistant: 'secondary.main', tool: 'warning.main' }
const lanes: RecordKind[] = ['user', 'assistant', 'tool']
const ledgerColumns = '72px 80px minmax(0, 1fr) 64px'

const ms = (e: TranscriptEntry) => Number(e.time)
const clock = (t: number) => new Date(t).toLocaleTimeString()
const firstLine = (s: string) => s.split('\n', 1)[0]

const formatDuration = (d: number) => {
  if (d < 1000) {
    return `${d}ms`
  }
  if (d < 60000) {
    return `${(d / 1000).toFixed(1)}s`
  }
  return `${Math.floor(d / 60000)}m ${String(Math.floor((d % 60000) / 1000)).padStart(2, '0')}s`
}

const durationText = (rec: Rec) => {
  if (rec.kind === 'user') {
    return ''
  }
  return rec.end === undefined ? 'Running…' : formatDuration(rec.end - rec.start)
}

const toolNameOf = (rec: Rec) => rec.entry.toolName || '(unknown)'

// buildRecords pairs the tool calls with the results. The start of the text of the agent is the time of the previous
// entry because the agent starts to generate it after that.
const buildRecords = (entries: TranscriptEntry[]): Rec[] => {
  const calls = new Set(entries.filter((e) => e.kind === TranscriptEntryKind.TOOL_USE).map((e) => e.toolUseId))
  const results = new Map(entries.filter((e) => e.kind === TranscriptEntryKind.TOOL_RESULT).map((e) => [e.toolUseId, e]))
  const recs: Rec[] = []
  let prev: number | undefined
  entries.forEach((e, key) => {
    const t = ms(e)
    switch (e.kind) {
      case TranscriptEntryKind.USER_PROMPT:
        recs.push({ key, kind: 'user', entry: e, start: t, end: t })
        break
      case TranscriptEntryKind.ASSISTANT_TEXT:
        recs.push({ key, kind: 'assistant', entry: e, start: prev ?? t, end: t })
        break
      case TranscriptEntryKind.TOOL_USE: {
        const result = results.get(e.toolUseId)
        recs.push({ key, kind: 'tool', entry: e, result, start: t, end: result ? ms(result) : undefined })
        break
      }
      case TranscriptEntryKind.TOOL_RESULT:
        if (!calls.has(e.toolUseId)) {
          recs.push({ key, kind: 'tool', entry: e, result: e, start: t, end: t })
        }
        break
    }
    prev = t
  })
  return recs
}

// buildTurns splits the records by the prompts of the user. The consecutive tool calls in a turn are grouped.
const buildTurns = (recs: Rec[]): Turn[] => {
  const turns: Turn[] = []
  for (const rec of recs) {
    let turn = turns[turns.length - 1]
    if (rec.kind === 'user' || !turn) {
      turn = { index: turns.length, items: [], start: rec.start, end: rec.start }
      turns.push(turn)
      if (rec.kind === 'user') {
        turn.prompt = rec
        continue
      }
    }
    turn.end = Math.max(turn.end, rec.end ?? rec.start)
    const last = turn.items[turn.items.length - 1]
    if (rec.kind !== 'tool') {
      turn.items.push({ type: 'record', rec })
    } else if (last?.type === 'calls') {
      last.recs.push(rec)
    } else if (last?.type === 'record' && last.rec.kind === 'tool') {
      turn.items[turn.items.length - 1] = { type: 'calls', key: last.rec.key, recs: [last.rec, rec] }
    } else {
      turn.items.push({ type: 'record', rec })
    }
  }
  return turns
}

const callsSummary = (recs: Rec[]) => {
  const counts = new Map<string, number>()
  for (const r of recs) {
    counts.set(toolNameOf(r), (counts.get(toolNameOf(r)) ?? 0) + 1)
  }
  return [...counts].map(([name, n]) => (n > 1 ? `${name} ×${n}` : name)).join(', ')
}

const Fold = ({ open }: { open: boolean }) => (open ? <ExpandMoreIcon sx={{ fontSize: 16 }} /> : <ChevronRightIcon sx={{ fontSize: 16 }} />)

const Overview = ({ recs, selected, onSelect }: { recs: Rec[]; selected?: number; onSelect: (rec: Rec) => void }) => {
  if (recs.length === 0) {
    return null
  }
  const t0 = recs[0].start
  const span = Math.max(Math.max(...recs.map((r) => r.end ?? r.start)) - t0, 1)
  const pos = (t: number) => `${((t - t0) / span) * 100}%`

  return (
    <Box sx={{ px: 1, py: 0.5, borderBottom: 1, borderColor: 'divider' }}>
      {lanes.map((lane) => (
        <Stack key={lane} direction="row" sx={{ alignItems: 'center', height: 14 }}>
          <Typography sx={{ width: 72, flexShrink: 0, fontSize: 11, color: 'text.secondary' }}>{kindLabels[lane]}</Typography>
          <Box sx={{ position: 'relative', flex: 1, height: 8 }}>
            {recs
              .filter((r) => r.kind === lane)
              .map((r) => (
                <Tooltip
                  key={r.key}
                  enterDelay={500}
                  title={`${lane === 'tool' ? toolNameOf(r) : kindLabels[lane]} · ${clock(r.start)}${r.end !== undefined && r.end !== r.start ? ` – ${clock(r.end)}` : ''} ${durationText(r)}`}
                >
                  <Box
                    onClick={() => onSelect(r)}
                    sx={{
                      position: 'absolute',
                      top: 0,
                      bottom: 0,
                      left: pos(r.start),
                      width: r.end === undefined ? 2 : `calc(${pos(r.end)} - ${pos(r.start)})`,
                      minWidth: 2,
                      cursor: 'pointer',
                      bgcolor: r.result?.isError ? 'error.main' : kindColors[lane],
                      opacity: selected === undefined || selected === r.key ? 1 : 0.5,
                      outline: selected === r.key ? 2 : 0,
                      outlineColor: 'text.primary',
                    }}
                  />
                </Tooltip>
              ))}
          </Box>
        </Stack>
      ))}
    </Box>
  )
}

const Row = ({ rec, selected, onSelect, indent = 0 }: { rec: Rec; selected: boolean; onSelect: () => void; indent?: number }) => {
  const summary = rec.kind === 'tool' ? rec.entry.text : firstLine(rec.entry.text)
  return (
    <Box
      data-record={rec.key}
      onClick={onSelect}
      sx={{
        display: 'grid',
        gridTemplateColumns: ledgerColumns,
        columnGap: 1,
        px: 1,
        py: 0.25,
        fontSize: 13,
        cursor: 'pointer',
        bgcolor: selected ? 'action.selected' : undefined,
        '&:hover': { bgcolor: selected ? 'action.selected' : 'action.hover' },
      }}
    >
      <Box sx={{ color: 'text.secondary', fontFamily: 'monospace', fontSize: 12 }}>{clock(rec.start)}</Box>
      <Box sx={{ color: kindColors[rec.kind], fontWeight: 'bold', pl: indent }}>{kindLabels[rec.kind]}</Box>
      <Box sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {rec.kind === 'tool' && (
          <Box component="span" sx={{ fontFamily: 'monospace', fontWeight: 'bold', mr: 1 }}>
            {toolNameOf(rec)}
          </Box>
        )}
        <Box component="span" sx={{ fontFamily: rec.kind === 'tool' ? 'monospace' : undefined }}>
          {summary}
        </Box>
      </Box>
      <Box sx={{ textAlign: 'right', fontFamily: 'monospace', fontSize: 12, color: rec.result?.isError ? 'error.main' : 'text.secondary' }}>
        {rec.result?.isError ? 'Error' : durationText(rec)}
      </Box>
    </Box>
  )
}

const Section = ({ title, children }: { title: string; children: ReactNode }) => (
  <Box sx={{ mt: 1.5 }}>
    <Typography variant="overline" color="text.secondary" sx={{ lineHeight: 1.5 }}>
      {title}
    </Typography>
    {children}
  </Box>
)

const Pre = ({ text, error }: { text: string; error?: boolean }) => (
  <Box
    component="pre"
    sx={{
      m: 0,
      p: 1,
      fontSize: 12,
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-all',
      bgcolor: 'action.hover',
      color: error ? 'error.main' : undefined,
    }}
  >
    {text}
  </Box>
)

const Inspector = ({ rec, onClose }: { rec: Rec; onClose: () => void }) => {
  const usage = rec.entry.usage
  const timing: [string, string][] = [['Start', clock(rec.start)]]
  if (rec.kind !== 'user') {
    timing.push(['End', rec.end === undefined ? '-' : clock(rec.end)], ['Duration', durationText(rec)])
  }
  const tokens: [string, bigint][] = usage
    ? [
        ['Input', usage.inputTokens],
        ['Output', usage.outputTokens],
        ['Cache read', usage.cacheReadInputTokens],
        ['Cache creation', usage.cacheCreationInputTokens],
      ]
    : []

  return (
    <Box sx={{ width: '40%', flexShrink: 0, borderLeft: 1, borderColor: 'divider', overflow: 'auto', p: 1 }}>
      <Stack direction="row" sx={{ alignItems: 'center' }}>
        <Typography sx={{ fontWeight: 'bold', color: kindColors[rec.kind], flex: 1 }}>
          {rec.kind === 'tool' ? `${kindLabels.tool} · ${toolNameOf(rec)}` : kindLabels[rec.kind]}
        </Typography>
        <IconButton size="small" onClick={onClose}>
          <CloseIcon fontSize="small" />
        </IconButton>
      </Stack>
      <Section title="Timing">
        <KeyValues rows={timing} />
      </Section>
      {tokens.length > 0 && (
        <Section title="Usage of the response">
          <KeyValues rows={tokens.map(([k, v]) => [k, v.toLocaleString()])} />
        </Section>
      )}
      {rec.kind === 'user' && (
        <Section title="Input">
          <Pre text={rec.entry.text} />
        </Section>
      )}
      {rec.kind === 'assistant' && (
        <Section title="Output">
          <Box sx={{ fontSize: 13, '& p': { my: 0.5 }, '& pre': { overflow: 'auto' } }}>
            <Markdown text={rec.entry.text} />
          </Box>
        </Section>
      )}
      {rec.kind === 'tool' && rec.entry.kind === TranscriptEntryKind.TOOL_USE && (
        <Section title="Input">
          <Pre text={rec.entry.text} />
        </Section>
      )}
      {rec.kind === 'tool' && (
        <Section title={rec.result?.isError ? 'Error' : 'Output'}>
          {rec.result ? <Pre text={rec.result.text} error={rec.result.isError} /> : <Typography color="text.secondary">Running…</Typography>}
        </Section>
      )}
    </Box>
  )
}

const KeyValues = ({ rows }: { rows: [string, string][] }) => (
  <Box sx={{ display: 'grid', gridTemplateColumns: 'max-content 1fr', columnGap: 2, fontSize: 13 }}>
    {rows.map(([k, v]) => (
      <Fragment key={k}>
        <Box sx={{ color: 'text.secondary' }}>{k}</Box>
        <Box sx={{ fontFamily: 'monospace' }}>{v}</Box>
      </Fragment>
    ))}
  </Box>
)

// TranscriptTimeline shows the session as a ledger of the records of the user, the agent and the tools grouped by the
// turns. The overview above the ledger plots the start and the duration of each record and the inspector shows the
// detail of the selected record. The ledger follows the end only while it is scrolled to the bottom.
export const TranscriptTimeline = ({ entries }: { entries: TranscriptEntry[] }) => {
  const ref = useRef<HTMLDivElement>(null)
  const atBottom = useRef(true)
  const [foldedTurns, setFoldedTurns] = useState<Set<number>>(() => new Set())
  const [foldedCalls, setFoldedCalls] = useState<Set<number>>(() => new Set())
  const [selected, setSelected] = useState<number>()

  useLayoutEffect(() => {
    const el = ref.current
    if (el && atBottom.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [entries])

  const recs = buildRecords(entries)
  const turns = buildTurns(recs)
  const selectedRec = recs.find((r) => r.key === selected)

  const toggle = (set: Set<number>, key: number) => {
    const next = new Set(set)
    if (!next.delete(key)) {
      next.add(key)
    }
    return next
  }
  const callGroups = turns.flatMap((t) => t.items.flatMap((i) => (i.type === 'calls' ? [i.key] : [])))

  const selectFromOverview = (rec: Rec) => {
    setSelected(rec.key)
    const turn = turns.find(
      (t) => t.prompt?.key === rec.key || t.items.some((i) => (i.type === 'record' ? i.rec.key === rec.key : i.recs.includes(rec))),
    )
    if (turn) {
      setFoldedTurns((s) => (s.has(turn.index) ? toggle(s, turn.index) : s))
      const group = turn.items.find((i) => i.type === 'calls' && i.recs.includes(rec))
      if (group?.type === 'calls') {
        setFoldedCalls((s) => (s.has(group.key) ? toggle(s, group.key) : s))
      }
    }
    requestAnimationFrame(() => ref.current?.querySelector(`[data-record="${rec.key}"]`)?.scrollIntoView({ block: 'nearest' }))
  }

  const select = (rec: Rec) => setSelected((s) => (s === rec.key ? undefined : rec.key))

  return (
    <Stack sx={{ height: '100%' }}>
      <Overview recs={recs} selected={selected} onSelect={selectFromOverview} />
      <Stack direction="row" spacing={1} sx={{ px: 1, py: 0.25, borderBottom: 1, borderColor: 'divider', alignItems: 'center' }}>
        <Typography variant="caption" color="text.secondary" sx={{ flex: 1 }}>
          {turns.length} turns · {recs.filter((r) => r.kind === 'tool').length} calls
        </Typography>
        <Button size="small" onClick={() => setFoldedTurns(foldedTurns.size === turns.length ? new Set() : new Set(turns.map((t) => t.index)))}>
          {foldedTurns.size === turns.length ? 'Expand turns' : 'Fold turns'}
        </Button>
        <Button size="small" onClick={() => setFoldedCalls(foldedCalls.size === callGroups.length ? new Set() : new Set(callGroups))}>
          {foldedCalls.size === callGroups.length && callGroups.length > 0 ? 'Expand calls' : 'Fold calls'}
        </Button>
      </Stack>
      <Stack direction="row" sx={{ flex: 1, minHeight: 0 }}>
        <Box
          ref={ref}
          onScroll={(e) => {
            const el = e.currentTarget
            atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 4
          }}
          sx={{ flex: 1, minWidth: 0, overflow: 'auto' }}
        >
          {turns.map((turn) => {
            const open = !foldedTurns.has(turn.index)
            const calls = turn.items.reduce((n, i) => n + (i.type === 'calls' ? i.recs.length : i.rec.kind === 'tool' ? 1 : 0), 0)
            return (
              <Box key={turn.index} sx={{ borderBottom: 1, borderColor: 'divider' }}>
                <Stack
                  direction="row"
                  spacing={1}
                  onClick={() => setFoldedTurns(toggle(foldedTurns, turn.index))}
                  sx={{ px: 0.5, py: 0.25, alignItems: 'center', cursor: 'pointer', bgcolor: 'action.hover', fontSize: 12, color: 'text.secondary' }}
                >
                  <Fold open={open} />
                  <Box sx={{ fontWeight: 'bold' }}>Turn {turn.index + 1}</Box>
                  <Box sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {turn.prompt ? firstLine(turn.prompt.entry.text) : ''}
                  </Box>
                  <Box sx={{ fontFamily: 'monospace' }}>
                    {calls} calls · {formatDuration(turn.end - turn.start)}
                  </Box>
                </Stack>
                {open && (
                  <>
                    {turn.prompt && <Row rec={turn.prompt} selected={selected === turn.prompt.key} onSelect={() => select(turn.prompt!)} />}
                    {turn.items.map((item) => {
                      if (item.type === 'record') {
                        return <Row key={item.rec.key} rec={item.rec} selected={selected === item.rec.key} onSelect={() => select(item.rec)} />
                      }
                      const callsOpen = !foldedCalls.has(item.key)
                      const start = item.recs[0].start
                      const ends = item.recs.map((r) => r.end)
                      const running = ends.some((e) => e === undefined)
                      return (
                        <Box key={item.key}>
                          <Box
                            onClick={() => setFoldedCalls(toggle(foldedCalls, item.key))}
                            sx={{
                              display: 'grid',
                              gridTemplateColumns: ledgerColumns,
                              columnGap: 1,
                              px: 1,
                              py: 0.25,
                              fontSize: 13,
                              cursor: 'pointer',
                              '&:hover': { bgcolor: 'action.hover' },
                            }}
                          >
                            <Box sx={{ color: 'text.secondary', fontFamily: 'monospace', fontSize: 12 }}>{clock(start)}</Box>
                            <Stack direction="row" sx={{ alignItems: 'center', color: kindColors.tool, fontWeight: 'bold' }}>
                              <Fold open={callsOpen} />
                              {item.recs.length} calls
                            </Stack>
                            <Box sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'text.secondary' }}>
                              {callsSummary(item.recs)}
                            </Box>
                            <Box sx={{ textAlign: 'right', fontFamily: 'monospace', fontSize: 12, color: 'text.secondary' }}>
                              {running ? 'Running…' : formatDuration(Math.max(...(ends as number[])) - start)}
                            </Box>
                          </Box>
                          {callsOpen &&
                            item.recs.map((r) => <Row key={r.key} rec={r} indent={2} selected={selected === r.key} onSelect={() => select(r)} />)}
                        </Box>
                      )
                    })}
                  </>
                )}
              </Box>
            )
          })}
        </Box>
        {selectedRec && <Inspector rec={selectedRec} onClose={() => setSelected(undefined)} />}
      </Stack>
    </Stack>
  )
}
