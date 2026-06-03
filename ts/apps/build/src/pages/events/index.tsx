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
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material'
import { Link } from '@tanstack/react-router'
import dayjs from 'dayjs'
import * as React from 'react'
import { StyledTableCell, StyledTableRow } from '../../components/Table.tsx'
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
                    <StyledTableCell>ID</StyledTableCell>
                    <StyledTableCell>Repository</StyledTableCell>
                    <StyledTableCell>Event</StyledTableCell>
                    <StyledTableCell>Action</StyledTableCell>
                    <StyledTableCell>State</StyledTableCell>
                    <StyledTableCell>Delivery ID</StyledTableCell>
                    <StyledTableCell>Created</StyledTableCell>
                    <StyledTableCell>Updated</StyledTableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {events.map((ev) => (
                    <StyledTableRow key={ev.id}>
                      <StyledTableCell>
                        <Link
                          to="/events/$eventId"
                          params={{ eventId: String(ev.id) }}
                        >
                          {ev.id}
                        </Link>
                      </StyledTableCell>
                      <StyledTableCell>
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
                      </StyledTableCell>
                      <StyledTableCell>{ev.eventType}</StyledTableCell>
                      <StyledTableCell>{ev.action || '—'}</StyledTableCell>
                      <StyledTableCell>
                        <Tooltip title={ev.lastError || ''} placement="top" arrow>
                          <Chip
                            label={ev.state}
                            size="small"
                            color={stateColor[ev.state] ?? 'default'}
                          />
                        </Tooltip>
                      </StyledTableCell>
                      <StyledTableCell>
                        <code style={{ fontSize: '0.75rem' }}>{ev.deliveryId}</code>
                      </StyledTableCell>
                      <StyledTableCell>{formatTime(ev.createdAt)}</StyledTableCell>
                      <StyledTableCell>{formatTime(ev.updatedAt)}</StyledTableCell>
                    </StyledTableRow>
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
