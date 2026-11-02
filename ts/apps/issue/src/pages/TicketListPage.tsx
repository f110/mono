import { useState } from 'react'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useQuery } from '@connectrpc/connect-query'
import { Link as RouterLink, useNavigate, useSearch } from '@tanstack/react-router'
import {
  Alert,
  Box,
  Chip,
  Link,
  MenuItem,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
  TextField,
  Typography,
} from '@mui/material'
import dayjs from 'dayjs'
import { Issue } from '../connect/issue_pb'
import { useMasters } from '../hooks/useMasters.ts'
import { priorityLabels, statusColors, statusLabels } from '../labels.ts'

const defaultFilter = 'status = OPEN OR status = IN_PROGRESS'
const defaultOrder = 'last_activity_at desc'

const columns: { label: string; field?: string }[] = [
  { label: '#', field: 'number' },
  { label: 'タイトル' },
  { label: 'Status', field: 'status' },
  { label: 'Priority', field: 'priority' },
  { label: 'Component', field: 'component' },
  { label: 'Assignee', field: 'assignee' },
  { label: '作成', field: 'created_at' },
  { label: '更新', field: 'last_activity_at' },
]

// withComponent returns the filter which is restricted to the component of name.
function withComponent(filter: string, name: string): string {
  if (name === '') {
    return filter
  }
  const restriction = `component = "${name.replaceAll('"', '\\"')}"`
  return filter.trim() === '' ? restriction : `(${filter}) AND ${restriction}`
}

export const TicketListPage = () => {
  const search = useSearch({ from: '/' })
  const navigate = useNavigate({ from: '/' })
  const filter = search.filter ?? defaultFilter
  const component = search.component ?? ''
  const orderBy = search.order_by ?? defaultOrder
  const [orderField, orderDirection] = orderBy.split(' ')
  const [input, setInput] = useState(filter)
  const { components, actorName, componentName } = useMasters()
  const { data, error } = useQuery(Issue.method.listTickets, { filter: withComponent(filter, component), orderBy })

  const sortBy = (field: string) => {
    const direction = field === orderField && orderDirection !== 'desc' ? ' desc' : ''
    void navigate({ search: (prev) => ({ ...prev, order_by: field + direction }) })
  }

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        <TextField
          select
          size="small"
          label="Component"
          value={component}
          onChange={(e) => void navigate({ search: (prev) => ({ ...prev, component: e.target.value || undefined }) })}
          sx={{ width: 200 }}
          slotProps={{ select: { displayEmpty: true }, inputLabel: { shrink: true } }}
        >
          <MenuItem value="">すべて</MenuItem>
          {components.map((c) => (
            <MenuItem key={c.id} value={c.name}>
              {c.name}
            </MenuItem>
          ))}
        </TextField>
        <Box
          component="form"
          sx={{ flex: 1, minWidth: 320 }}
          onSubmit={(e) => {
            e.preventDefault()
            void navigate({ search: (prev) => ({ ...prev, filter: input }) })
          }}
        >
          <TextField
            label="フィルタ (AIP-160)"
            placeholder="status = OPEN AND priority <= P1 クラッシュ"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            fullWidth
            size="small"
          />
        </Box>
      </Stack>
      {error && <Alert severity="error">{error.message}</Alert>}
      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              {columns.map((c) => (
                <TableCell key={c.label} sx={{ whiteSpace: 'nowrap' }}>
                  {c.field ? (
                    <TableSortLabel
                      active={c.field === orderField}
                      direction={c.field === orderField && orderDirection === 'desc' ? 'desc' : 'asc'}
                      onClick={() => sortBy(c.field!)}
                    >
                      {c.label}
                    </TableSortLabel>
                  ) : (
                    c.label
                  )}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {data?.tickets.map((t) => (
              <TableRow key={t.number} hover>
                <TableCell>{t.number}</TableCell>
                <TableCell>
                  <Link component={RouterLink} to="/$number" params={{ number: String(t.number) }}>
                    {t.title}
                  </Link>
                </TableCell>
                <TableCell>
                  <Chip label={statusLabels[t.status]} size="small" sx={statusColors[t.status]} />
                </TableCell>
                <TableCell>{t.priority ? priorityLabels[t.priority] : ''}</TableCell>
                <TableCell>{t.componentId ? componentName(t.componentId) : ''}</TableCell>
                <TableCell>{t.assigneeId ? actorName(t.assigneeId) : ''}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{t.createdAt && dayjs(timestampDate(t.createdAt)).format('YYYY-MM-DD')}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{t.lastActivityAt && dayjs(timestampDate(t.lastActivityAt)).fromNow()}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      {data && data.tickets.length === 0 && <Typography color="text.secondary">チケットはありません</Typography>}
    </Stack>
  )
}
