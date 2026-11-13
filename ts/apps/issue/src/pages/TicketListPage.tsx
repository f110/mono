import { useState } from 'react'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useQuery } from '@connectrpc/connect-query'
import { skipToken } from '@tanstack/react-query'
import {
  Link as RouterLink,
  useNavigate,
  useSearch,
} from '@tanstack/react-router'
import {
  Alert,
  Box,
  Checkbox,
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
  TablePagination,
  TableRow,
  TableSortLabel,
  TextField,
  Typography,
} from '@mui/material'
import dayjs from 'dayjs'
import { type Ticket, Issue, TicketStatus } from '../connect/issue_pb'
import { CopyTicketNumberButton } from '../components/CopyButton.tsx'
import { RelativeTime } from '../components/RelativeTime.tsx'
import { useMasters } from '../hooks/useMasters.ts'
import { priorityLabels, statusColors, statusLabels, statuses, typeLabels } from '../labels.ts'

const defaultStatuses = [TicketStatus.OPEN, TicketStatus.NEED_INVESTIGATION, TicketStatus.IN_PROGRESS].map((s) => TicketStatus[s])
const statusLabelByName = Object.fromEntries(statuses.map((s) => [TicketStatus[s], statusLabels[s]]))
const defaultOrder = 'last_activity_at desc'
const pageSize = 100
const wideOnlySx = { display: { xs: 'none', sm: 'table-cell' } }

const columns: { label: string; field?: string; grow?: boolean; wideOnly?: boolean }[] = [
  { label: '#', field: 'number' },
  { label: 'Component', field: 'component', wideOnly: true },
  { label: 'タイトル', grow: true },
  { label: 'Status', field: 'status', wideOnly: true },
  { label: 'Type', wideOnly: true },
  { label: 'Priority', field: 'priority', wideOnly: true },
  { label: 'Assignee', field: 'assignee', wideOnly: true },
  { label: '作成', field: 'created_at', wideOnly: true },
  { label: '更新', field: 'last_activity_at', wideOnly: true },
]

// and returns the filter which joins non-empty conditions with AND.
function and(...conditions: string[]): string {
  return conditions
    .filter((c) => c.trim() !== '')
    .map((c) => `(${c})`)
    .join(' AND ')
}

export const TicketListPage = () => {
  const search = useSearch({ from: '/' })
  const navigate = useNavigate({ from: '/' })
  const filter = search.filter ?? ''
  const selectedStatuses = search.status ?? defaultStatuses
  const component = search.component ?? ''
  const orderBy = search.order_by ?? defaultOrder
  const [orderField, orderDirection] = orderBy.split(' ')
  const [input, setInput] = useState(filter)
  const { components } = useMasters()
  const pages = search.pages ?? []
  const separateP0 = filter === '' && search.status === undefined
  const condition = and(
    filter,
    selectedStatuses.map((s) => `status = ${s}`).join(' OR '),
    component === '' ? '' : `component = "${component.replaceAll('"', '\\"')}"`,
  )
  const { data, error } = useQuery(Issue.method.listTickets, {
    filter: separateP0 ? and(condition, 'priority != P0') : condition,
    orderBy,
    pageSize,
    pageToken: pages.at(-1) ?? '',
  })
  const { data: p0, error: p0Error } = useQuery(
    Issue.method.listTickets,
    separateP0 && pages.length === 0 ? { filter: and(condition, 'priority = P0'), orderBy } : skipToken,
  )

  const sortBy = (field: string) => {
    const direction =
      field === orderField && orderDirection !== 'desc' ? ' desc' : ''
    void navigate({
      search: (prev) => ({ ...prev, order_by: field + direction, pages: undefined }),
    })
  }
  const movePage = (next: string[]) => {
    void navigate({ search: (prev) => ({ ...prev, pages: next.length > 0 ? next : undefined }) })
  }

  return (
    <Stack spacing={2}>
      <Stack
        direction="row"
        spacing={2}
        useFlexGap
        sx={{ alignItems: 'center', flexWrap: 'wrap' }}
      >
        <TextField
          select
          size="small"
          label="Component"
          value={component}
          onChange={(e) =>
            void navigate({
              search: (prev) => ({
                ...prev,
                component: e.target.value || undefined,
                pages: undefined,
              }),
            })
          }
          sx={{ width: 200 }}
          slotProps={{
            select: { displayEmpty: true },
            inputLabel: { shrink: true },
          }}
        >
          <MenuItem value="">すべて</MenuItem>
          {components.map((c) => (
            <MenuItem key={c.id} value={c.name}>
              {c.name}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          select
          size="small"
          label="Status"
          value={selectedStatuses}
          onChange={(e) => {
            const value = e.target.value as unknown as string[]
            void navigate({ search: (prev) => ({ ...prev, status: value, pages: undefined }) })
          }}
          sx={{ width: 240 }}
          slotProps={{
            select: {
              multiple: true,
              displayEmpty: true,
              renderValue: (selected) =>
                (selected as string[]).length === 0
                  ? 'すべて'
                  : (selected as string[]).map((s) => statusLabelByName[s]).join(', '),
            },
            inputLabel: { shrink: true },
          }}
        >
          {statuses.map((s) => (
            <MenuItem key={s} value={TicketStatus[s]}>
              <Checkbox size="small" checked={selectedStatuses.includes(TicketStatus[s])} sx={{ p: 0, mr: 1 }} />
              {statusLabels[s]}
            </MenuItem>
          ))}
        </TextField>
        <Box
          component="form"
          sx={{ flex: 1, minWidth: 240 }}
          onSubmit={(e) => {
            e.preventDefault()
            void navigate({ search: (prev) => ({ ...prev, filter: input, pages: undefined }) })
          }}
        >
          <TextField
            label="フィルタ"
            placeholder="priority <= P1 クラッシュ"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            fullWidth
            size="small"
          />
        </Box>
      </Stack>
      {error && <Alert severity="error">{error.message}</Alert>}
      {p0Error && <Alert severity="error">{p0Error.message}</Alert>}
      {p0 && p0.tickets.length > 0 && (
        <Stack spacing={1}>
          <Typography variant="h6">P0</Typography>
          <TicketTable tickets={p0.tickets} orderField={orderField} orderDirection={orderDirection} onSort={sortBy} />
        </Stack>
      )}
      <TicketTable tickets={data?.tickets ?? []} orderField={orderField} orderDirection={orderDirection} onSort={sortBy} />
      {data && data.tickets.length === 0 && (
        <Typography color="text.secondary">チケットはありません</Typography>
      )}
      <TablePagination
        component="div"
        count={data && !data.nextPageToken ? pages.length * pageSize + data.tickets.length : -1}
        page={pages.length}
        rowsPerPage={pageSize}
        rowsPerPageOptions={[]}
        onPageChange={(_, page) => movePage(page > pages.length ? [...pages, data!.nextPageToken] : pages.slice(0, -1))}
        slotProps={{ actions: { nextButton: { disabled: !data?.nextPageToken } } }}
      />
    </Stack>
  )
}

const TicketTable = ({
  tickets,
  orderField,
  orderDirection,
  onSort,
}: {
  tickets: Ticket[]
  orderField: string
  orderDirection: string | undefined
  onSort: (field: string) => void
}) => {
  const { actorName, componentName } = useMasters()
  return (
    <TableContainer component={Paper}>
      <Table size="small" sx={{ '& th, & td': { whiteSpace: 'nowrap', px: { xs: 1, sm: 2 } }, '& .grow': { width: '100%', whiteSpace: 'normal' } }}>
        <TableHead>
          <TableRow sx={{ bgcolor: 'grey.100' }}>
            {columns.map((c) => (
              <TableCell key={c.label} className={c.grow ? 'grow' : undefined} sx={c.wideOnly ? wideOnlySx : undefined}>
                {c.field ? (
                  <TableSortLabel
                    active={c.field === orderField}
                    direction={c.field === orderField && orderDirection === 'desc' ? 'desc' : 'asc'}
                    onClick={() => onSort(c.field!)}
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
          {tickets.map((t) => (
            <TableRow key={t.number} hover>
              <TableCell>
                {t.number}
                <CopyTicketNumberButton number={t.number} />
              </TableCell>
              <TableCell sx={wideOnlySx}>{t.componentId ? componentName(t.componentId) : ''}</TableCell>
              <TableCell className="grow">
                <Link component={RouterLink} to="/$number" params={{ number: String(t.number) }}>
                  {t.title}
                </Link>
                <Stack direction="row" spacing={1} sx={{ display: { xs: 'flex', sm: 'none' }, alignItems: 'center', mt: 0.5 }}>
                  <Chip label={statusLabels[t.status]} size="small" sx={statusColors[t.status]} />
                  <Typography variant="body2" color="text.secondary">
                    <RelativeTime time={t.lastActivityAt} />
                  </Typography>
                </Stack>
              </TableCell>
              <TableCell sx={wideOnlySx}>
                <Chip label={statusLabels[t.status]} size="small" sx={statusColors[t.status]} />
              </TableCell>
              <TableCell sx={wideOnlySx}>{t.type ? typeLabels[t.type] : ''}</TableCell>
              <TableCell sx={wideOnlySx}>{t.priority ? priorityLabels[t.priority] : ''}</TableCell>
              <TableCell sx={wideOnlySx}>{t.assigneeId ? actorName(t.assigneeId) : ''}</TableCell>
              <TableCell sx={wideOnlySx}>
                {t.createdAt && dayjs(timestampDate(t.createdAt)).format('YYYY-MM-DD')}
              </TableCell>
              <TableCell sx={wideOnlySx}>
                <RelativeTime time={t.lastActivityAt} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}
