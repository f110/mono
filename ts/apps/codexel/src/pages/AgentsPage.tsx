import { useEffect, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  Alert,
  Box,
  Button,
  ButtonGroup,
  Chip,
  Divider,
  List,
  ListItemButton,
  ListItemText,
  ListSubheader,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AgentOutput } from '../components/AgentOutput.tsx'
import { RevisionDetail } from '../components/RevisionDetail.tsx'
import { StartAgentDialog } from '../components/StartAgentDialog.tsx'
import { TranscriptTimeline } from '../components/TranscriptTimeline.tsx'
import { AgentService, AgentStatus, ReviewService, RevisionService, type Agent } from '../connect/codexel_pb.js'
import { useAgents } from '../hooks/useAgents.ts'
import { useDiffNavigation } from '../hooks/useDiffNavigation.ts'
import { useOperation } from '../hooks/useOperation.ts'
import { useTranscript } from '../hooks/useTranscript.ts'
import { transport } from '../providers/AppProvider.tsx'

const reviewClient = createClient(ReviewService, transport)

const outputRefreshInterval = 1000
// outputHistoryLines is the number of the lines including the history. herdr captures the history only while the
// agent is idle.
const outputHistoryLines = 2000
// refreshDelayAfterInput is the delay to read the output after sending the input because the agent redraws the screen
// asynchronously.
const refreshDelayAfterInput = 300
const inputHeight = 40

const statusChip: Record<AgentStatus, { label: string; color: 'default' | 'primary' | 'warning' | 'success' }> = {
  [AgentStatus.UNSPECIFIED]: { label: 'unknown', color: 'default' },
  [AgentStatus.IDLE]: { label: 'idle', color: 'default' },
  [AgentStatus.WORKING]: { label: 'working', color: 'primary' },
  [AgentStatus.BLOCKED]: { label: 'blocked', color: 'warning' },
  [AgentStatus.DONE]: { label: 'done', color: 'success' },
  [AgentStatus.UNKNOWN]: { label: 'unknown', color: 'default' },
}

// statusOrder is the order of the groups in the list. The agents waiting for the user come first.
const statusOrder: AgentStatus[] = [AgentStatus.BLOCKED, AgentStatus.DONE, AgentStatus.IDLE, AgentStatus.WORKING, AgentStatus.UNKNOWN]

const keys: { label: string; key: string }[] = [
  { label: '↑', key: 'up' },
  { label: '↓', key: 'down' },
  { label: 'Enter', key: 'enter' },
  { label: 'Esc', key: 'esc' },
  { label: 'Ctrl+C', key: 'ctrl+c' },
]

const StatusChip = ({ status }: { status: AgentStatus }) => (
  <Chip size="small" label={statusChip[status].label} color={statusChip[status].color} variant="outlined" />
)

const groupByStatus = (agents: Agent[]): [AgentStatus, Agent[]][] => {
  const normalize = (s: AgentStatus) => (statusOrder.includes(s) ? s : AgentStatus.UNKNOWN)
  return statusOrder
    .map((s): [AgentStatus, Agent[]] => [s, agents.filter((a) => normalize(a.status) === s)])
    .filter(([, list]) => list.length > 0)
}

// AgentCommits lists the commits created by the session of the agent and shows the description and the diff of the
// selected one. The list is read again every time the repository changes.
const AgentCommits = ({ agent }: { agent: Agent }) => {
  const { data, error, refetch } = useQuery(RevisionService.method.listAgentCommits, { cwd: agent.cwd, kind: agent.kind, sessionId: agent.sessionId })
  const path = data?.repository?.path
  const operation = useOperation(path)
  useEffect(() => {
    refetch()
  }, [operation, refetch])
  const [selectedChange, setSelectedChange] = useState<string>()
  const detailRef = useRef<HTMLDivElement>(null)
  const { split, preview } = useDiffNavigation(detailRef)
  const commits = data?.commits ?? []
  const selected = commits.find((c) => c.changeId === selectedChange) ?? commits[0]
  const navigate = useNavigate()
  const [reviewError, setReviewError] = useState<string>()

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  if (data && commits.length === 0) {
    return (
      <Typography color="text.secondary" sx={{ p: 2 }}>
        No commits of this session in {path}
      </Typography>
    )
  }
  // startReview starts the review of all commits in order from the oldest because the list shows the newest first.
  const startReview = async () => {
    if (!path) return
    try {
      const res = await reviewClient.startReview({ path, changeIds: commits.map((c) => c.changeId).reverse() })
      navigate({ to: '/reviews/$reviewId', params: { reviewId: res.review!.id } })
    } catch (e) {
      setReviewError(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <Stack direction="row" sx={{ height: '100%' }}>
      <List dense sx={{ width: 360, flexShrink: 0, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
        <Box sx={{ px: 1, pb: 1 }}>
          <Button size="small" variant="outlined" fullWidth disabled={!path || commits.length === 0} onClick={startReview}>
            Review these commits
          </Button>
          {reviewError && (
            <Alert severity="error" onClose={() => setReviewError(undefined)} sx={{ mt: 1 }}>
              {reviewError}
            </Alert>
          )}
        </Box>
        {commits.map((c) => (
          <ListItemButton key={c.commitId} selected={c === selected} onClick={() => setSelectedChange(c.changeId)}>
            <ListItemText
              primary={c.description || '(no description set)'}
              secondary={`${c.changeId.slice(0, 8)} · ${new Date(Number(c.timestamp) * 1000).toLocaleString()}`}
              slotProps={{ primary: { noWrap: true }, secondary: { sx: { fontFamily: 'monospace' } } }}
            />
          </ListItemButton>
        ))}
      </List>
      <Box ref={detailRef} sx={{ flex: 1, minWidth: 0, overflow: 'auto' }}>
        {path && selected && <RevisionDetail key={selected.commitId} path={path} commit={selected} split={split} preview={preview} />}
      </Box>
    </Stack>
  )
}

const AgentDetail = ({ agent }: { agent: Agent }) => {
  // The output changes continuously only while the agent is working or waiting for the answer.
  const polling = agent.status === AgentStatus.WORKING || agent.status === AgentStatus.BLOCKED
  const { data, error, refetch } = useQuery(
    AgentService.method.readOutput,
    { paneId: agent.paneId, lines: outputHistoryLines },
    { refetchInterval: polling ? outputRefreshInterval : false },
  )
  useEffect(() => {
    refetch()
  }, [agent.status, refetch])
  const refetchAfterInput = () => setTimeout(() => refetch(), refreshDelayAfterInput)

  const sendPrompt = useMutation(AgentService.method.sendPrompt)
  const sendKeys = useMutation(AgentService.method.sendKeys)
  const focus = useMutation(AgentService.method.focusAgent)
  const [prompt, setPrompt] = useState('')
  const [tab, setTab] = useState<'output' | 'commits' | 'timeline'>('output')
  // The transcript is available only for Claude Code.
  const hasTranscript = agent.kind === 'claude' && agent.sessionId !== ''
  const { transcript, disabled: transcriptDisabled, error: transcriptError } = useTranscript(hasTranscript ? agent.sessionId : '')
  const showTimeline = hasTranscript && !transcriptDisabled

  const submit = () => {
    if (!prompt) {
      return
    }
    sendPrompt.mutate(
      { paneId: agent.paneId, text: prompt },
      {
        onSuccess: () => {
          setPrompt('')
          refetchAfterInput()
        },
      },
    )
  }
  const shownError = error ?? sendPrompt.error ?? sendKeys.error ?? focus.error ?? transcriptError
  const shownTab = tab === 'timeline' && !showTimeline ? 'output' : tab

  return (
    <Stack sx={{ height: '100%', minWidth: 0 }}>
      <Stack direction="row" spacing={1} sx={{ p: 1, alignItems: 'center' }}>
        <StatusChip status={agent.status} />
        <Typography variant="subtitle1">{agent.title || agent.kind}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {agent.cwd}
        </Typography>
        <Typography variant="caption" color="text.secondary" title="Agent / Agent-Session">
          {agent.kind} / {agent.sessionId}
        </Typography>
        <Button size="small" onClick={() => focus.mutate({ paneId: agent.paneId })}>
          Focus in herdr
        </Button>
      </Stack>
      {showTimeline && transcript?.title && (
        <Stack sx={{ px: 1, pb: 1, minWidth: 0 }}>
          <Typography variant="body2" noWrap title="Theme of the session">
            {transcript.title}
          </Typography>
          {transcript.lastPrompt && (
            <Typography variant="caption" color="text.secondary" noWrap title={transcript.lastPrompt}>
              Last prompt: {transcript.lastPrompt}
            </Typography>
          )}
        </Stack>
      )}
      {shownError && <Alert severity="error">{shownError.message}</Alert>}
      <Tabs value={shownTab} onChange={(_, v) => setTab(v)} sx={{ minHeight: 36, borderBottom: 1, borderColor: 'divider' }}>
        <Tab value="output" label="Output" sx={{ minHeight: 36 }} />
        <Tab value="commits" label="Commits" disabled={!agent.sessionId} sx={{ minHeight: 36 }} />
        {showTimeline && <Tab value="timeline" label="Timeline (experimental)" sx={{ minHeight: 36 }} />}
      </Tabs>
      <Box sx={{ flex: 1, minHeight: 0 }}>
        {shownTab === 'output' && <AgentOutput text={data?.text ?? ''} />}
        {shownTab === 'commits' && <AgentCommits agent={agent} />}
        {shownTab === 'timeline' && <TranscriptTimeline entries={transcript?.entries ?? []} />}
      </Box>
      <Stack direction="row" spacing={1} sx={{ p: 1, alignItems: 'flex-end' }}>
        <TextField
          multiline
          maxRows={8}
          fullWidth
          size="small"
          placeholder="Prompt (Cmd/Ctrl+Enter to send)"
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              submit()
            }
          }}
        />
        <Button variant="contained" onClick={submit} disabled={!prompt || sendPrompt.isPending} sx={{ height: inputHeight, flexShrink: 0 }}>
          Send
        </Button>
        <ButtonGroup sx={{ height: inputHeight, flexShrink: 0 }}>
          {keys.map((k) => (
            <Button
              key={k.key}
              sx={{ whiteSpace: 'nowrap' }}
              onClick={() => sendKeys.mutate({ paneId: agent.paneId, keys: [k.key] }, { onSuccess: refetchAfterInput })}
            >
              {k.label}
            </Button>
          ))}
        </ButtonGroup>
      </Stack>
    </Stack>
  )
}

export const AgentsPage = () => {
  const { agents, error } = useAgents()
  const { pane } = useSearch({ from: '/agents' })
  const navigate = useNavigate({ from: '/agents' })
  const [startOpen, setStartOpen] = useState(false)
  const selected = agents?.find((a) => a.paneId === pane)
  const select = (paneId: string) => navigate({ search: { pane: paneId } })

  return (
    <Box sx={{ display: 'flex', height: '100%' }}>
      <Box sx={{ width: 320, flexShrink: 0, overflow: 'auto', borderRight: 1, borderColor: 'divider' }}>
        <Box sx={{ p: 1 }}>
          <Button fullWidth variant="outlined" onClick={() => setStartOpen(true)}>
            Start agent
          </Button>
        </Box>
        {error && <Alert severity="error">{error.message}</Alert>}
        <Divider />
        {agents &&
          groupByStatus(agents).map(([status, list]) => (
            <List key={status} dense subheader={<ListSubheader>{statusChip[status].label}</ListSubheader>}>
              {list.map((a) => (
                <ListItemButton key={a.paneId} selected={a.paneId === pane} onClick={() => select(a.paneId)}>
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                        <StatusChip status={a.status} />
                        <span>{a.workspaceLabel || a.workspaceId}</span>
                        <Typography variant="body2" color="text.secondary" component="span" noWrap>
                          {a.title || a.kind}
                        </Typography>
                      </Stack>
                    }
                    secondary={a.cwd}
                    slotProps={{ secondary: { noWrap: true } }}
                  />
                </ListItemButton>
              ))}
            </List>
          ))}
      </Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        {selected ? (
          <AgentDetail key={selected.paneId} agent={selected} />
        ) : (
          <Typography color="text.secondary" sx={{ p: 2 }}>
            {pane ? 'The agent is not found' : 'Select an agent'}
          </Typography>
        )}
      </Box>
      <StartAgentDialog open={startOpen} onClose={() => setStartOpen(false)} onStarted={(a) => select(a.paneId)} />
    </Box>
  )
}
