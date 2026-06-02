import { timestampDate } from '@bufbuild/protobuf/wkt'
import {
  Box,
  Chip,
  Container,
  Link as MuiLink,
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
import { Link } from '@tanstack/react-router'
import dayjs from 'dayjs'
import * as React from 'react'
import type { GithubEvent } from '../../model/msg_pb'
import { useListGithubEvents } from '../../hooks/useListGithubEvents'
import { stateColor } from './stateColor'

const formatTime = (ts?: GithubEvent['createdAt']): string =>
  ts ? dayjs(timestampDate(ts)).format('YYYY-MM-DD HH:mm:ss') : ''

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
                    <TableCell>Delivery ID</TableCell>
                    <TableCell>Created</TableCell>
                    <TableCell>Updated</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {events.map((ev) => (
                    <TableRow key={ev.id}>
                      <TableCell>
                        <Link
                          to="/events/$eventId"
                          params={{ eventId: String(ev.id) }}
                        >
                          {ev.id}
                        </Link>
                      </TableCell>
                      <TableCell>
                        {ev.repository ? (
                          ev.repositoryUrl ? (
                            <MuiLink
                              href={ev.repositoryUrl}
                              target="_blank"
                              rel="noopener"
                              underline="hover"
                            >
                              {ev.repository}
                            </MuiLink>
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
