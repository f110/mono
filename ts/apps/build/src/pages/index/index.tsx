import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useQuery, useSuspenseQuery } from '@connectrpc/connect-query'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import SyncIcon from '@mui/icons-material/Sync'
import { useState } from 'react'
import * as React from 'react'
import {
  Button,
  Stack,
  Box,
  Container,
  TableContainer,
  Table,
  TableBody,
  TableHead,
  TableRow,
  Paper,
  IconButton,
  Typography,
  Breadcrumbs,
  InputLabel,
  Select,
  MenuItem,
  FormControl,
  type SelectChangeEvent,
  TablePagination,
} from '@mui/material'
import CheckIcon from '@mui/icons-material/Check'
import RefreshIcon from '@mui/icons-material/Refresh'
import ErrorIcon from '@mui/icons-material/Error'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { LogModal } from '../../components/LogModal.tsx'
import { ManifestModal } from '../../components/ManifestModal.tsx'
import { RunTaskModal } from '../../components/RunTaskModal.tsx'
import { StyledTableCell, StyledTableRow } from '../../components/Table.tsx'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { BFF, type BFFTask } from '../../connect/bff_pb'
import { useListRepositories } from '../../hooks/useListRepositories.ts'
import dayjs from 'dayjs'
import { useRestartTask } from '../../hooks/useRestartTask.ts'
import { Route } from '../../routes'
import { formatDuration } from '../../utils/duration.ts'

type TaskResultRowProps = {
  task: BFFTask
  onRestart: (id: number) => void
  openManifestModal: (manifest: string) => void
  openLogModal: (taskId: number) => void
}

const TaskResultRow: React.FC<TaskResultRowProps> = ({
  task,
  onRestart,
  openManifestModal,
  openLogModal,
}) => {
  const start = task.startAt
    ? dayjs(timestampDate(task.startAt)).format('YYYY-MM-DD HH:mm:ss')
    : ''

  return (
    <StyledTableRow>
      <StyledTableCell align="center">
        {task.success ? (
          <CheckIcon color="success" />
        ) : task.finishedAt ? (
          <ErrorIcon color="error" />
        ) : (
          <SyncIcon color="warning" />
        )}
      </StyledTableCell>
      <StyledTableCell>
        <Link to={'/task/$taskId'} params={{ taskId: String(task.id) }}>
          {task.id}
        </Link>
      </StyledTableCell>
      <StyledTableCell>
        <Link
          to="."
          search={({}) => ({
            repository_id: task.repository?.id,
          })}
        >
          {task.repository?.name}
        </Link>
        @
        <Link to="." href={task.revisionUrl}>
          {task.revision.length === 40
            ? task.revision.slice(0, 8)
            : task.revision}
        </Link>
      </StyledTableCell>
      <StyledTableCell>{task.jobName}</StyledTableCell>
      <StyledTableCell>{task.command}</StyledTableCell>
      <StyledTableCell>
        {task.finishedAt && (
          <Link
            to="."
            onClick={() => {
              openLogModal(task.id)
            }}
          >
            text
          </Link>
        )}
      </StyledTableCell>
      <StyledTableCell>
        {task.startAt && (
          <Link
            to="."
            onClick={() => {
              openManifestModal(task.manifest)
            }}
          >
            yaml
          </Link>
        )}
      </StyledTableCell>
      <StyledTableCell>{task.via}</StyledTableCell>
      <StyledTableCell>{start}</StyledTableCell>
      <StyledTableCell>{formatDuration(task.duration)}</StyledTableCell>
      <StyledTableCell>
        <IconButton onClick={() => onRestart(task.id)}>
          <RefreshIcon color="warning" />
        </IconButton>
      </StyledTableCell>
    </StyledTableRow>
  )
}

export const IndexPage: React.FC = () => {
  const [runTaskModal, setRunTaskModal] = useState<boolean>(false)
  const runTaskModalOpen = () => setRunTaskModal(true)
  const runTaskModalClose = () => setRunTaskModal(false)

  const [manifestModal, setManifestModal] = useState<boolean>(false)
  const [manifest, setManifest] = useState<string>('')
  const manifestModalOpen = (manifest: string) => {
    setManifest(manifest)
    setManifestModal(true)
  }
  const manifestModalClose = () => {
    setManifest('')
    setManifestModal(false)
  }

  const [logModal, setLogModal] = useState(false)
  const [taskId, setTaskId] = useState<number>(0)
  const logModalOpen = (taskId: number) => {
    setTaskId(taskId)
    setLogModal(true)
  }
  const logModalClose = () => {
    setTaskId(0)
    setLogModal(false)
  }

  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(1)
  const [pageToken, setPageToken] = useState<string | undefined>(undefined)
  const [pageTokenHistory, setPageTokenHistory] = useState<string[]>([])
  const handleChangePage = (
    event: React.MouseEvent<HTMLButtonElement> | null,
    newPage: number,
  ) => {
    if (page + 1 == newPage) {
      setPageTokenHistory([...pageTokenHistory, pageToken])
      setPageToken(tasks.data?.nextPageToken)
    } else if (page - 1 == newPage) {
      const token = pageTokenHistory.pop()
      setPageToken(token)
      setPageTokenHistory([...pageTokenHistory])
    }
    setPage(newPage)
  }
  const handleChangeRowsPerPage = (
    event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>,
  ) => {
    setRowsPerPage(parseInt(event.target.value, 10))
    setPage(0)
    setPageToken(undefined)
    setPageTokenHistory([])
  }

  const params = useSearch({ strict: false })
  const tasks = useQuery(BFF.method.listTasks, {
    repositoryId:
      'repository_id' in params ? Number(params['repository_id']) : undefined,
    pageSize: rowsPerPage,
    pageToken: pageToken,
  })
  const repositories = useListRepositories()
  const { mutate: restartTask } = useRestartTask()
  const onRestart = (id: number) => {
    restartTask({ taskId: id })
  }
  const [repository, setRepository] = useState<string>(
    'repository_id' in params ? String(params['repository_id']) : '',
  )
  const navigate = useNavigate({ from: Route.fullPath })
  const onSelectRepository = async (select: SelectChangeEvent) => {
    setRepository(select.target.value)
    if (select.target.value === '') {
      navigate({ search: () => ({}) })
    } else {
      navigate({
        search: () => ({
          repository_id: Number(select.target.value),
        }),
      })
    }
  }

  return (
    <Container maxWidth="xl">
      <Box
        sx={{
          width: '100%',
          maxWidth: { sm: '100%', md: '1700px' },
        }}
      >
        <Stack spacing={2}>
          <Breadcrumbs
            aria-label="breadcrumb"
            separator={<NavigateNextIcon fontSize="small" />}
          >
            <Typography>All tasks</Typography>
            {repository !== '' && (
              <Typography>
                {repositories.find((v) => v.id === Number(repository))?.name}
              </Typography>
            )}
          </Breadcrumbs>

          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              color="primary"
              startIcon={<PlayArrowIcon />}
              onClick={runTaskModalOpen}
              sx={{ textTransform: 'none' }}
            >
              Run
            </Button>

            <FormControl
              sx={{ m: 1, minWidth: '100px', textAlign: 'left' }}
              variant="standard"
            >
              <InputLabel id="repository">Repository...</InputLabel>
              <Select
                labelId="repository"
                label="Repository..."
                onChange={onSelectRepository}
                value={repository}
                autoWidth
              >
                <MenuItem value="">All</MenuItem>
                {repositories.map((repository) => (
                  <MenuItem value={repository.id}>{repository.name}</MenuItem>
                ))}
              </Select>
            </FormControl>
          </Stack>

          <TableContainer component={Paper}>
            <Table sx={{ width: '100%' }} aria-label="customized table">
              <TableHead>
                <TableRow>
                  <StyledTableCell></StyledTableCell>
                  <StyledTableCell>#</StyledTableCell>
                  <StyledTableCell>Commit</StyledTableCell>
                  <StyledTableCell>Job</StyledTableCell>
                  <StyledTableCell>Command</StyledTableCell>
                  <StyledTableCell>Log</StyledTableCell>
                  <StyledTableCell>Manifest</StyledTableCell>
                  <StyledTableCell>Trigger</StyledTableCell>
                  <StyledTableCell>Start at</StyledTableCell>
                  <StyledTableCell>Duration</StyledTableCell>
                  <StyledTableCell />
                </TableRow>
              </TableHead>
              <TableBody>
                {tasks.data?.tasks.map((task) => (
                  <TaskResultRow
                    key={String(task.id)}
                    task={task}
                    onRestart={onRestart}
                    openManifestModal={manifestModalOpen}
                    openLogModal={logModalOpen}
                  />
                ))}
              </TableBody>
            </Table>
            <TablePagination
              component="div"
              count={-1}
              page={page}
              onPageChange={handleChangePage}
              rowsPerPage={rowsPerPage}
              onRowsPerPageChange={handleChangeRowsPerPage}
              slotProps={{
                actions: {
                  nextButton: {
                    disabled: tasks.data?.nextPageToken === '',
                  },
                },
              }}
            />
          </TableContainer>
        </Stack>
      </Box>

      <RunTaskModal
        open={runTaskModal}
        onClose={runTaskModalClose}
        repositories={repositories}
      />

      <LogModal open={logModal} onClose={logModalClose} taskId={taskId} />

      <ManifestModal
        open={manifestModal}
        onClose={manifestModalClose}
        manifest={manifest}
      />
    </Container>
  )
}
