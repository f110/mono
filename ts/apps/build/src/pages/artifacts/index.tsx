import { timestampDate } from '@bufbuild/protobuf/wkt'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DescriptionIcon from '@mui/icons-material/Description'
import InventoryIcon from '@mui/icons-material/Inventory2'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import TerminalIcon from '@mui/icons-material/Terminal'
import {
  Avatar,
  Box,
  Breadcrumbs,
  Chip,
  Container,
  IconButton,
  InputAdornment,
  Paper,
  Snackbar,
  Stack,
  Table,
  TableBody,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { Link, useSearch } from '@tanstack/react-router'
import dayjs from 'dayjs'
import * as React from 'react'
import { useMemo, useState } from 'react'
import { StyledTableCell, StyledTableRow } from '../../components/Table.tsx'
import type { BFFArtifact } from '../../connect/bff_pb'
import { useGetServerInfo } from '../../hooks/useGetServerInfo.ts'
import { useListArtifacts } from '../../hooks/useListArtifacts.ts'
import { useListRepositories } from '../../hooks/useListRepositories.ts'
import { ArtifactType } from '../../model/msg_pb'

const TYPE_META: Record<
  ArtifactType,
  { label: string; icon: React.ReactElement; color: string }
> = {
  [ArtifactType.CONTAINER_IMAGE]: {
    label: 'Container image',
    icon: <InventoryIcon fontSize="small" />,
    color: '#1976d2',
  },
  [ArtifactType.BINARY]: {
    label: 'Binary',
    icon: <TerminalIcon fontSize="small" />,
    color: '#6d4c41',
  },
  [ArtifactType.K8S_MANIFEST]: {
    label: 'K8s manifest',
    icon: <DescriptionIcon fontSize="small" />,
    color: '#2e7d32',
  },
}

const formatSize = (b: bigint): string => {
  const n = Number(b)
  if (n >= 1048576) return `${(n / 1048576).toFixed(1)} MiB`
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KiB`
  return `${n} B`
}

const shortDigest = (d: string): string => {
  const h = d.startsWith('sha256:') ? d.slice(7) : d
  return h.length > 16 ? `sha256:${h.slice(0, 6)}…${h.slice(-5)}` : d
}

const producedAt = (a: BFFArtifact): string =>
  a.createdAt
    ? dayjs(timestampDate(a.createdAt)).format('YYYY-MM-DD HH:mm:ss')
    : ''

type SortKey = 'size' | 'produced'

export const ArtifactsPage: React.FC = () => {
  const params = useSearch({ strict: false })
  const taskId = 'task_id' in params ? Number(params['task_id']) : undefined
  const repositoryId =
    'repository_id' in params ? Number(params['repository_id']) : undefined
  const revision =
    'revision' in params ? String(params['revision']) : undefined
  const isCommitScope = taskId === undefined

  const { data } = useListArtifacts({ taskId, repositoryId, revision })
  const repositories = useListRepositories()
  const { config } = useGetServerInfo()
  const host = config?.artifactRegistryHost ?? ''

  const repository = repositories.find((v) => v.id === repositoryId)
  const artifacts = useMemo(() => data?.artifacts ?? [], [data])

  const [search, setSearch] = useState('')
  const [types, setTypes] = useState<Set<ArtifactType>>(new Set())
  const [sortKey, setSortKey] = useState<SortKey>('produced')
  const [asc, setAsc] = useState(false)

  const toggleType = (t: ArtifactType) => {
    const next = new Set(types)
    next.has(t) ? next.delete(t) : next.add(t)
    setTypes(next)
  }
  const onSort = (key: SortKey) => {
    if (sortKey === key) setAsc(!asc)
    else {
      setSortKey(key)
      setAsc(false)
    }
  }

  const counts = useMemo(() => {
    const c = new Map<ArtifactType, number>()
    for (const a of artifacts) c.set(a.type, (c.get(a.type) ?? 0) + 1)
    return c
  }, [artifacts])
  const totalSize = useMemo(
    () => artifacts.reduce((s, a) => s + a.size, 0n),
    [artifacts],
  )
  const taskCount = useMemo(
    () => new Set(artifacts.map((a) => a.taskId)).size,
    [artifacts],
  )

  const rows = useMemo(() => {
    const q = search.trim().toLowerCase()
    const filtered = artifacts.filter(
      (a) =>
        (types.size === 0 || types.has(a.type)) &&
        (q === '' ||
          `${a.name} ${a.registry}`.toLowerCase().includes(q)),
    )
    const sorted = [...filtered].sort((x, y) => {
      const r =
        sortKey === 'size'
          ? Number(x.size - y.size)
          : producedAt(x).localeCompare(producedAt(y))
      return asc ? r : -r
    })
    return sorted
  }, [artifacts, search, types, sortKey, asc])

  const [snack, setSnack] = useState<string | null>(null)
  const copy = (text: string, message: string) => {
    navigator.clipboard?.writeText(text)
    setSnack(message)
  }

  return (
    <Container maxWidth="xl">
      <Box sx={{ width: '100%' }}>
        <Stack spacing={2}>
          <Breadcrumbs
            aria-label="breadcrumb"
            separator={<NavigateNextIcon fontSize="small" />}
          >
            {isCommitScope ? (
              [
                <Link key="repos" color="inherit" to="/repositories">
                  Repositories
                </Link>,
                <Typography key="repo">{repository?.name}</Typography>,
                <Typography key="rev" sx={{ fontFamily: 'monospace' }}>
                  {revision && revision.length === 40
                    ? revision.slice(0, 8)
                    : revision}
                </Typography>,
                <Typography key="cur" sx={{ color: 'text.primary' }}>
                  Artifacts
                </Typography>,
              ]
            ) : (
              [
                <Link key="tasks" color="inherit" to="/">
                  Tasks
                </Link>,
                <Link
                  key="task"
                  color="inherit"
                  to="/task/$taskId"
                  params={{ taskId: String(taskId) }}
                >
                  {taskId}
                </Link>,
                <Typography key="cur" sx={{ color: 'text.primary' }}>
                  Artifacts
                </Typography>,
              ]
            )}
          </Breadcrumbs>

          <Paper sx={{ p: 2 }}>
            <Typography variant="h6">
              {isCommitScope ? (
                <>
                  {repository?.name}{' '}
                  <Box
                    component="span"
                    sx={{ fontFamily: 'monospace', color: 'text.secondary' }}
                  >
                    @ {revision?.slice(0, 8)}
                  </Box>
                </>
              ) : (
                <>Task #{taskId}</>
              )}
            </Typography>
            <Stack
              direction="row"
              spacing={3}
              sx={{ mt: 1, color: 'text.secondary', flexWrap: 'wrap' }}
            >
              <span>
                <strong>{artifacts.length}</strong> artifacts
              </span>
              <span>
                Total <strong>{formatSize(totalSize)}</strong>
              </span>
              {isCommitScope && (
                <span>
                  from <strong>{taskCount}</strong> tasks
                </span>
              )}
            </Stack>
          </Paper>

          <Stack direction="row" spacing={2} sx={{ flexWrap: 'wrap' }} useFlexGap>
            <TextField
              size="small"
              placeholder="name / registry で絞り込み"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchIcon fontSize="small" />
                    </InputAdornment>
                  ),
                },
              }}
              sx={{ minWidth: 260 }}
            />
            <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
              {(
                [
                  ArtifactType.CONTAINER_IMAGE,
                  ArtifactType.BINARY,
                  ArtifactType.K8S_MANIFEST,
                ] as ArtifactType[]
              ).map((t) => (
                <Chip
                  key={t}
                  icon={TYPE_META[t].icon}
                  label={`${TYPE_META[t].label} (${counts.get(t) ?? 0})`}
                  variant={types.has(t) ? 'filled' : 'outlined'}
                  color={types.has(t) ? 'primary' : 'default'}
                  onClick={() => toggleType(t)}
                />
              ))}
            </Stack>
          </Stack>

          <TableContainer component={Paper}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <StyledTableCell>Artifact</StyledTableCell>
                  <StyledTableCell>Type</StyledTableCell>
                  <StyledTableCell>Registry</StyledTableCell>
                  <StyledTableCell>Digest / Tag</StyledTableCell>
                  <StyledTableCell sortDirection={sortKey === 'size' ? (asc ? 'asc' : 'desc') : false}>
                    <TableSortLabel
                      active={sortKey === 'size'}
                      direction={sortKey === 'size' && asc ? 'asc' : 'desc'}
                      onClick={() => onSort('size')}
                      sx={{ color: 'inherit !important' }}
                    >
                      Size
                    </TableSortLabel>
                  </StyledTableCell>
                  {isCommitScope && <StyledTableCell>Task</StyledTableCell>}
                  <StyledTableCell sortDirection={sortKey === 'produced' ? (asc ? 'asc' : 'desc') : false}>
                    <TableSortLabel
                      active={sortKey === 'produced'}
                      direction={sortKey === 'produced' && asc ? 'asc' : 'desc'}
                      onClick={() => onSort('produced')}
                      sx={{ color: 'inherit !important' }}
                    >
                      Produced at
                    </TableSortLabel>
                  </StyledTableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((a) => {
                  const meta = TYPE_META[a.type]
                  return (
                    <StyledTableRow key={a.id}>
                      <StyledTableCell>
                        <Stack direction="row" spacing={1.5} alignItems="center">
                          <Avatar
                            variant="rounded"
                            sx={{ bgcolor: meta.color, width: 32, height: 32 }}
                          >
                            {meta.icon}
                          </Avatar>
                          <span>{a.name}</span>
                        </Stack>
                      </StyledTableCell>
                      <StyledTableCell sx={{ color: 'text.secondary' }}>
                        {meta.label}
                      </StyledTableCell>
                      <StyledTableCell sx={{ fontFamily: 'monospace' }}>
                        {host ? `${host}/${a.registry}` : a.registry}
                      </StyledTableCell>
                      <StyledTableCell>
                        <Stack direction="row" spacing={0.5} alignItems="center">
                          <Tooltip title={a.digest}>
                            <Box component="span" sx={{ fontFamily: 'monospace' }}>
                              {shortDigest(a.digest)}
                            </Box>
                          </Tooltip>
                          <Tooltip title="digest をコピー">
                            <IconButton
                              size="small"
                              onClick={() => copy(a.digest, 'digest をコピーしました')}
                            >
                              <ContentCopyIcon sx={{ fontSize: 15 }} />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                        {a.tag ? (
                          <Chip
                            size="small"
                            label={a.tag}
                            color="success"
                            variant="outlined"
                            sx={{ mt: 0.5, fontFamily: 'monospace', height: 20 }}
                          />
                        ) : (
                          <Box
                            component="span"
                            sx={{
                              display: 'block',
                              mt: 0.5,
                              fontSize: '0.72rem',
                              fontStyle: 'italic',
                              color: 'text.disabled',
                            }}
                          >
                            untagged
                          </Box>
                        )}
                      </StyledTableCell>
                      <StyledTableCell sx={{ whiteSpace: 'nowrap' }}>
                        {formatSize(a.size)}
                      </StyledTableCell>
                      {isCommitScope && (
                        <StyledTableCell>
                          <Link
                            to="/task/$taskId"
                            params={{ taskId: String(a.taskId) }}
                            style={{ fontFamily: 'monospace' }}
                          >
                            #{a.taskId}
                          </Link>
                        </StyledTableCell>
                      )}
                      <StyledTableCell
                        sx={{ whiteSpace: 'nowrap', color: 'text.secondary' }}
                      >
                        {producedAt(a)}
                      </StyledTableCell>
                    </StyledTableRow>
                  )
                })}
                {rows.length === 0 && (
                  <TableRow>
                    <StyledTableCell
                      colSpan={isCommitScope ? 7 : 6}
                      align="center"
                      sx={{ py: 6, color: 'text.secondary' }}
                    >
                      {artifacts.length === 0
                        ? 'このスコープで収集されたアーティファクトはありません'
                        : '条件に一致するアーティファクトがありません'}
                    </StyledTableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Stack>
      </Box>

      <Snackbar
        open={snack !== null}
        autoHideDuration={1600}
        onClose={() => setSnack(null)}
        message={snack ?? ''}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />
    </Container>
  )
}
