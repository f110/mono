import { useState } from 'react'
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

const statusChip: Record<AgentStatus, { label: string; color: 'default' | 'primary' | 'warning' | 'success' }> = {
  [AgentStatus.UNSPECIFIED]: { label: 'unknown', color: 'default' },
  [AgentStatus.IDLE]: { label: 'idle', color: 'default' },
  [AgentStatus.WORKING]: { label: 'working', color: 'primary' },
  [AgentStatus.BLOCKED]: { label: 'blocked', color: 'warning' },
  [AgentStatus.DONE]: { label: 'done', color: 'success' },
  [AgentStatus.UNKNOWN]: { label: 'unknown', color: 'default' },
}

const StatusChip = ({ status }: { status: AgentStatus }) => (
  <Chip size="small" label={statusChip[status].label} color={statusChip[status].color} variant="outlined" />
)

const groupByWorkspace = (agents: Agent[]): [string, Agent[]][] => {
  const groups = new Map<string, Agent[]>()
  for (const a of agents) {
    const label = a.workspaceLabel || a.workspaceId
    groups.set(label, [...(groups.get(label) ?? []), a])
  }
  return [...groups.entries()]
}

const AgentDetail = ({ agent }: { agent: Agent }) => {
  const { data, error } = useQuery(AgentService.method.readOutput, { paneId: agent.paneId }, { refetchInterval: outputRefreshInterval })
  const sendPrompt = useMutation(AgentService.method.sendPrompt)
  const sendKeys = useMutation(AgentService.method.sendKeys)
  const focus = useMutation(AgentService.method.focusAgent)
  const [prompt, setPrompt] = useState('')

  const submit = () => {
    if (!prompt) {
      return
    }
    sendPrompt.mutate({ paneId: agent.paneId, text: prompt }, { onSuccess: () => setPrompt('') })
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
        <Button variant="contained" onClick={submit} disabled={!prompt || sendPrompt.isPending}>
          Send
        </Button>
        <ButtonGroup size="small">
          {['Enter', 'Escape', 'C-c'].map((k) => (
            <Button key={k} onClick={() => sendKeys.mutate({ paneId: agent.paneId, keys: [k] })}>
              {k}
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
          groupByWorkspace(agents).map(([label, list]) => (
            <List key={label} dense subheader={<ListSubheader>{label}</ListSubheader>}>
              {list.map((a) => (
                <ListItemButton key={a.paneId} selected={a.paneId === pane} onClick={() => select(a.paneId)}>
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                        <StatusChip status={a.status} />
                        <span>{a.title || a.kind}</span>
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
