import { timestampDate } from '@bufbuild/protobuf/wkt'
import { Code } from '@connectrpc/connect'
import { useSuspenseQuery } from '@connectrpc/connect-query'
import CheckIcon from '@mui/icons-material/Check'
import ErrorIcon from '@mui/icons-material/Error'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import SyncIcon from '@mui/icons-material/Sync'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Breadcrumbs,
  Button,
  Container,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Typography,
} from '@mui/material'
import { styled } from '@mui/material/styles'
import { Link, notFound, useParams } from '@tanstack/react-router'
import dayjs from 'dayjs'
import * as React from 'react'
import { useState } from 'react'
import { LogModal } from '../../components/LogModal.tsx'
import { ManifestModal } from '../../components/ManifestModal.tsx'
import { BFF } from '../../connect/bff_pb'
import { useCompleteTask } from '../../hooks/useCompleteTask.ts'
import { useForceStopTask } from '../../hooks/useForceStopTask.ts'
import { useGetServerInfo } from '../../hooks/useGetServerInfo.ts'
import { useLiveDuration } from '../../hooks/useLiveDuration.ts'
import { useRestartTask } from '../../hooks/useRestartTask.ts'
import { type BuildMetrics, TestStatus } from '../../model/msg_pb'

const DefinitionTableCell = styled(TableCell)(({ theme }) => ({
  '&:first-child': {
    backgroundColor: theme.palette.action.hover,
  },
}))

const formatMs = (ms: bigint): string => {
  const n = Number(ms)
  return n < 1000 ? `${n} ms` : `${(n / 1000).toFixed(2)} s`
}

const formatBytes = (b: bigint): string =>
  `${(Number(b) / (1024 * 1024)).toFixed(1)} MiB`

// cacheHitRate reports the remote cache hit rate. CI jobs always start with a cold
// local cache, so every action is either a remote cache hit or actually executed.
const cacheHitRate = (bm: BuildMetrics): string => {
  const hits = Number(bm.remoteCacheHits)
  const total = hits + Number(bm.actionsExecuted)
  const pct = total > 0 ? ((hits / total) * 100).toFixed(1) : '0.0'
  return `${pct}% (${hits} / ${total})`
}

export const TaskPage: React.FC = () => {
  const { taskId } = useParams({ strict: false })
  const {
    data: tasks,
    error: taskError,
    isSuccess,
  } = useSuspenseQuery(
    BFF.method.listTasks,
    {
      taskId: Number(taskId),
    },
    {
      retry: (_failureCount, error) => {
        if (error.code == Code.NotFound) {
          return false
        }
        return true
      },
    },
  )
  if (taskError?.code == Code.NotFound || !tasks) {
    throw notFound()
  }
  const task = tasks.tasks[0]
  const start = task?.startAt
    ? dayjs(timestampDate(task.startAt)).format('YYYY-MM-DD HH:mm:ss')
    : ''
  const duration = useLiveDuration({
    startAt: task?.startAt,
    finishedAt: task?.finishedAt,
    duration: task?.duration,
  })
  const [manifestModal, setManifestModal] = useState<boolean>(false)
  const [manifest, setManifest] = useState<string>('')
  const manifestModalClose = () => {
    setManifest('')
    setManifestModal(false)
  }
  const manifestModalOpen = (manifest: string) => {
    setManifest(manifest)
    setManifestModal(true)
  }
  const [logModal, setLogModal] = useState(false)

  const { mutate: restartTask } = useRestartTask()
  const handleRerun = () => {
    restartTask({ taskId: Number(taskId) })
  }
  const { mutate: forceStopTask, isPending: isForceStopping } =
    useForceStopTask()
  const handleForceStop = () => {
    forceStopTask({ taskId: Number(taskId) })
  }
  const { config } = useGetServerInfo()
  const { mutate: completeTask, isPending: isCompleting } = useCompleteTask()
  const handleComplete = (success: boolean) => {
    completeTask({ taskId: Number(taskId), success })
  }

  if (!isSuccess) {
    return <></>
  }

  return (
    <Container maxWidth="xl">
      <Box sx={{ width: '100%' }}>
        <Stack spacing={2}>
          <Breadcrumbs
            aria-label="breadcrumb"
            separator={<NavigateNextIcon fontSize="small" />}
          >
            <Link color="inherit" to="/">
              {task?.repository?.name}
            </Link>
            <Typography>Tasks</Typography>
            <Typography sx={{ color: 'text.primary' }}>{taskId}</Typography>
          </Breadcrumbs>

          <TableContainer component={Paper}>
            <Table>
              <TableBody>
                <TableRow>
                  <DefinitionTableCell>ID</DefinitionTableCell>
                  <DefinitionTableCell>{taskId}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Succeeded</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task?.success ? (
                      <CheckIcon color="success" />
                    ) : task?.finishedAt ? (
                      <ErrorIcon color="error" />
                    ) : (
                      <SyncIcon color="warning" />
                    )}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Job</DefinitionTableCell>
                  <DefinitionTableCell>{task?.jobName}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Revision</DefinitionTableCell>
                  <DefinitionTableCell>
                    <Link to="." href={task?.revisionUrl}>
                      {task?.revision}
                    </Link>
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Trunk</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task?.isTrunk ? 'Yes' : 'No'}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Started at</DefinitionTableCell>
                  <DefinitionTableCell>{start}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Test report</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task?.executedTestsCount > 0 ? (
                      <Accordion>
                        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                          {`${task?.succeededTestsCount} / ${task?.executedTestsCount}`}
                        </AccordionSummary>
                        <AccordionDetails>
                          <List>
                            {task?.testReports.map((v) => (
                              <ListItem>
                                <ListItemIcon>
                                  {v.status === TestStatus.PASSED && (
                                    <CheckIcon color="success" />
                                  )}
                                  {v.status === TestStatus.FLAKY && (
                                    <CheckIcon color="warning" />
                                  )}
                                  {v.status === TestStatus.FAILED && (
                                    <ErrorIcon color="error" />
                                  )}
                                </ListItemIcon>
                                <ListItemText>{v.label}</ListItemText>
                              </ListItem>
                            ))}
                          </List>
                        </AccordionDetails>
                      </Accordion>
                    ) : (
                      <p>There is no executed test</p>
                    )}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Duration</DefinitionTableCell>
                  <DefinitionTableCell>{duration}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Node</DefinitionTableCell>
                  <DefinitionTableCell>{task?.node}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Trigger</DefinitionTableCell>
                  <DefinitionTableCell>{task?.via}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Bazel version</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task?.bazelVersion}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Container</DefinitionTableCell>
                  <DefinitionTableCell>{task?.container}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>CPU / Memory Limit</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task?.cpuLimit} / {task?.memoryLimit}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Log</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task.finishedAt && (
                      <Link to="." onClick={() => setLogModal(true)}>
                        text
                      </Link>
                    )}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Job manifest</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task.startAt && (
                      <Link
                        to="."
                        onClick={() => {
                          manifestModalOpen(task ? task.manifest : '')
                        }}
                      >
                        yaml
                      </Link>
                    )}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Artifacts</DefinitionTableCell>
                  <DefinitionTableCell>
                    <Link
                      to="/artifacts"
                      search={{ task_id: Number(taskId) }}
                    >
                      view
                    </Link>
                  </DefinitionTableCell>
                </TableRow>
              </TableBody>
            </Table>
          </TableContainer>

          {task?.buildMetrics && (
            <>
              <Typography variant="h6">Build metrics</Typography>
              <TableContainer component={Paper}>
                <Table>
                  <TableBody>
                    <TableRow>
                      <DefinitionTableCell>
                        Remote cache hit rate
                      </DefinitionTableCell>
                      <DefinitionTableCell>
                        {cacheHitRate(task.buildMetrics)}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>Wall / CPU time</DefinitionTableCell>
                      <DefinitionTableCell>
                        {formatMs(task.buildMetrics.wallTimeMs)} /{' '}
                        {formatMs(task.buildMetrics.cpuTimeMs)}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>
                        Analysis / Execution phase
                      </DefinitionTableCell>
                      <DefinitionTableCell>
                        {formatMs(task.buildMetrics.analysisPhaseTimeMs)} /{' '}
                        {formatMs(task.buildMetrics.executionPhaseTimeMs)}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>
                        Actions (created / executed)
                      </DefinitionTableCell>
                      <DefinitionTableCell>
                        {`${task.buildMetrics.actionsCreated} / ${task.buildMetrics.actionsExecuted}`}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>
                        Graph action count
                      </DefinitionTableCell>
                      <DefinitionTableCell>
                        {`${task.buildMetrics.graphActionCount}`}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>
                        Targets / Packages
                      </DefinitionTableCell>
                      <DefinitionTableCell>
                        {`${task.buildMetrics.targetsConfigured} / ${task.buildMetrics.packagesLoaded}`}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>Skyframe nodes</DefinitionTableCell>
                      <DefinitionTableCell>
                        {`${task.buildMetrics.skyframeNodeCount}`}
                      </DefinitionTableCell>
                    </TableRow>
                    <TableRow>
                      <DefinitionTableCell>
                        Artifacts read / seen / from cache
                      </DefinitionTableCell>
                      <DefinitionTableCell>
                        {formatBytes(task.buildMetrics.sourceArtifactsReadBytes)}{' '}
                        /{' '}
                        {formatBytes(task.buildMetrics.outputArtifactsSeenBytes)}{' '}
                        /{' '}
                        {formatBytes(
                          task.buildMetrics.outputArtifactsFromCacheBytes,
                        )}
                      </DefinitionTableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )}

          <Stack direction="row" sx={{ width: '100%' }} spacing={2}>
            <Button
              variant="contained"
              color="warning"
              sx={{ textTransform: 'none' }}
              onClick={handleRerun}
            >
              Rerun
            </Button>
            {!task.finishedAt && (
              <Button
                variant="contained"
                color="error"
                onClick={handleForceStop}
                loading={isForceStopping}
                loadingPosition="end"
              >
                Force Stop
              </Button>
            )}
            {/* Dev-only: only shown when the builder emulates the cluster with kwok. */}
            {config?.emulateJobControl && !task.finishedAt && (
              <>
                <Button
                  variant="outlined"
                  color="success"
                  onClick={() => handleComplete(true)}
                  loading={isCompleting}
                  loadingPosition="end"
                >
                  Complete (dev)
                </Button>
                <Button
                  variant="outlined"
                  color="error"
                  onClick={() => handleComplete(false)}
                  loading={isCompleting}
                  loadingPosition="end"
                >
                  Fail (dev)
                </Button>
              </>
            )}
          </Stack>
        </Stack>
      </Box>

      <LogModal
        open={logModal}
        onClose={() => {
          setLogModal(false)
        }}
        taskId={taskId ? Number(taskId) : 0}
      />
      <ManifestModal
        open={manifestModal}
        onClose={manifestModalClose}
        manifest={manifest}
      />
    </Container>
  )
}
