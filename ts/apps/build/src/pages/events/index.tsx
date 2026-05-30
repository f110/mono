import { timestampDate } from '@bufbuild/protobuf/wkt'
import {
  Box,
  Chip,
  Container,
  Link,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material'
import dayjs from 'dayjs'
import * as React from 'react'
import type { GithubEvent } from '../../model/msg_pb'
import { useListGithubEvents } from '../../hooks/useListGithubEvents'

// stateColor maps the reconciler state name to a MUI palette color. Falls
// back to "default" for terminal states that the user does not need to act on.
const stateColor: Record<
  string,
  'default' | 'primary' | 'info' | 'success' | 'error' | 'warning'
> = {
  PENDING: 'info',
  PROCESSING: 'primary',
  SUCCEEDED: 'success',
  FAILED: 'error',
  EXPIRED: 'warning',
  SKIPPED: 'default',
}

const formatTime = (ts?: GithubEvent['createdAt']): string =>
  ts ? dayjs(timestampDate(ts)).format('YYYY-MM-DD HH:mm:ss') : ''

// prettyJSON returns status pretty-printed for the tooltip. Invalid JSON is
// returned unchanged so we never throw inside a render path.
const prettyJSON = (raw: string): string => {
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

export const EventsPage: React.FC = () => {
  const events = useListGithubEvents()

  return (
    <Container maxWidth={false}>
      <Box sx={{ width: '100%' }}>
        <Stack spacing={2}>
          <Typography variant="h5">GitHub Events</Typography>
          {events.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No webhook deliveries recorded yet.
            </Typography>
          ) : (
            <TableContainer component={Paper}>
              <Table size="small" aria-label="github events">
                <TableHead>
                  <TableRow>
                    <TableCell>ID</TableCell>
                    <TableCell>Repository</TableCell>
                    <TableCell>Event</TableCell>
                    <TableCell>Action</TableCell>
                    <TableCell>State</TableCell>
                    <TableCell>Status</TableCell>
                    <TableCell>Delivery ID</TableCell>
                    <TableCell>Created</TableCell>
                    <TableCell>Updated</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {events.map((ev) => (
                    <TableRow key={ev.id}>
                      <TableCell>{ev.id}</TableCell>
                      <TableCell>
                        {ev.repository ? (
                          ev.repositoryUrl ? (
                            <Link
                              href={ev.repositoryUrl}
                              target="_blank"
                              rel="noopener"
                              underline="hover"
                            >
                              {ev.repository}
                            </Link>
                          ) : (
                            ev.repository
                          )
                        ) : (
                          '—'
                        )}
                      </TableCell>
                      <TableCell>{ev.eventType}</TableCell>
                      <TableCell>{ev.action || '—'}</TableCell>
                      <TableCell>
                        <Tooltip title={ev.lastError || ''} placement="top" arrow>
                          <Chip
                            label={ev.state}
                            size="small"
                            color={stateColor[ev.state] ?? 'default'}
                          />
                        </Tooltip>
                      </TableCell>
                      <TableCell sx={{ maxWidth: 320 }}>
                        {ev.status ? (
                          <Tooltip
                            title={
                              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
                                {prettyJSON(ev.status)}
                              </pre>
                            }
                            placement="top"
                            arrow
                          >
                            <Box
                              component="code"
                              sx={{
                                display: 'block',
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                                whiteSpace: 'nowrap',
                                fontSize: '0.75rem',
                              }}
                            >
                              {ev.status}
                            </Box>
                          </Tooltip>
                        ) : (
                          '—'
                        )}
                      </TableCell>
                      <TableCell>
                        <code style={{ fontSize: '0.75rem' }}>{ev.deliveryId}</code>
                      </TableCell>
                      <TableCell>{formatTime(ev.createdAt)}</TableCell>
                      <TableCell>{formatTime(ev.updatedAt)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </Stack>
      </Box>
    </Container>
  )
}
