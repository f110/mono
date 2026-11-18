import { useEffect, useState } from 'react'
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
  TextField,
  Typography,
} from '@mui/material'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AgentOutput } from '../components/AgentOutput.tsx'
import { StartAgentDialog } from '../components/StartAgentDialog.tsx'
import { AgentService, AgentStatus, type Agent } from '../connect/codexel_pb.js'
import { useAgents } from '../hooks/useAgents.ts'

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
  const shownError = error ?? sendPrompt.error ?? sendKeys.error ?? focus.error

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
      {shownError && <Alert severity="error">{shownError.message}</Alert>}
      <Box sx={{ flex: 1, minHeight: 0 }}>
        <AgentOutput text={data?.text ?? ''} />
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
