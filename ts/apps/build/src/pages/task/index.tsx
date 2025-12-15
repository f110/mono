import { timestampDate } from '@bufbuild/protobuf/wkt'
import { Code } from '@connectrpc/connect'
import { useSuspenseQuery } from '@connectrpc/connect-query'
import CheckIcon from '@mui/icons-material/Check'
import ErrorIcon from '@mui/icons-material/Error'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
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
import { TestStatus } from '../../model/msg_pb'
import { formatDuration } from '../../utils/duration.ts'

const DefinitionTableCell = styled(TableCell)(({ theme }) => ({
  '&:first-child': {
    backgroundColor: theme.palette.action.hover,
  },
}))

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
                  <DefinitionTableCell>Status</DefinitionTableCell>
                  <DefinitionTableCell>
                    {task?.success ? (
                      <CheckIcon color="success" />
                    ) : (
                      <ErrorIcon color="error" />
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
                  <DefinitionTableCell>
                    {formatDuration(task?.duration)}
                  </DefinitionTableCell>
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
                    <Link to="." onClick={() => setLogModal(true)}>
                      text
                    </Link>
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Job manifest</DefinitionTableCell>
                  <DefinitionTableCell>
                    <Link
                      to="."
                      onClick={() => {
                        manifestModalOpen(task ? task.manifest : '')
                      }}
                    >
                      yaml
                    </Link>
                  </DefinitionTableCell>
                </TableRow>
              </TableBody>
            </Table>
          </TableContainer>
          <Stack direction="row" sx={{ width: '100%' }}>
            <Button
              variant="contained"
              color="warning"
              sx={{ textTransform: 'none' }}
            >
              Rerun
            </Button>
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
