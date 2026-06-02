import { timestampDate } from '@bufbuild/protobuf/wkt'
import { Code } from '@connectrpc/connect'
import { useSuspenseQuery } from '@connectrpc/connect-query'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import {
  Box,
  Breadcrumbs,
  Chip,
  Container,
  Link as MuiLink,
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
import { BFF } from '../../connect/bff_pb'
import type { GithubEvent } from '../../model/msg_pb'
import { stateColor } from './stateColor'

const DefinitionTableCell = styled(TableCell)(({ theme }) => ({
  '&:first-child': {
    backgroundColor: theme.palette.action.hover,
  },
}))

const formatTime = (ts?: GithubEvent['createdAt']): string =>
  ts ? dayjs(timestampDate(ts)).format('YYYY-MM-DD HH:mm:ss') : ''

export const EventPage: React.FC = () => {
  const { eventId } = useParams({ strict: false })
  const {
    data: res,
    error,
    isSuccess,
  } = useSuspenseQuery(
    BFF.method.listGithubEvents,
    {
      eventId: Number(eventId),
    },
    {
      retry: (_failureCount, err) => {
        if (err.code == Code.NotFound) {
          return false
        }
        return true
      },
    },
  )
  if (error?.code == Code.NotFound || !res) {
    throw notFound()
  }
  const event = res.events[0]
  if (!event) {
    throw notFound()
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
            <Link color="inherit" to="/events">
              Events
            </Link>
            <Typography sx={{ color: 'text.primary' }}>{eventId}</Typography>
          </Breadcrumbs>

          <TableContainer component={Paper}>
            <Table>
              <TableBody>
                <TableRow>
                  <DefinitionTableCell>ID</DefinitionTableCell>
                  <DefinitionTableCell>{eventId}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Repository</DefinitionTableCell>
                  <DefinitionTableCell>
                    {event.repository ? (
                      event.repositoryUrl ? (
                        <MuiLink
                          href={event.repositoryUrl}
                          target="_blank"
                          rel="noopener"
                          underline="hover"
                        >
                          {event.repository}
                        </MuiLink>
                      ) : (
                        event.repository
                      )
                    ) : (
                      '—'
                    )}
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Event</DefinitionTableCell>
                  <DefinitionTableCell>{event.eventType}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Action</DefinitionTableCell>
                  <DefinitionTableCell>{event.action || '—'}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>State</DefinitionTableCell>
                  <DefinitionTableCell>
                    <Chip
                      label={event.state}
                      size="small"
                      color={stateColor[event.state] ?? 'default'}
                    />
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Delivery ID</DefinitionTableCell>
                  <DefinitionTableCell>
                    <code style={{ fontSize: '0.75rem' }}>{event.deliveryId}</code>
                  </DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Created</DefinitionTableCell>
                  <DefinitionTableCell>{formatTime(event.createdAt)}</DefinitionTableCell>
                </TableRow>
                <TableRow>
                  <DefinitionTableCell>Updated</DefinitionTableCell>
                  <DefinitionTableCell>{formatTime(event.updatedAt)}</DefinitionTableCell>
                </TableRow>
                {event.lastError && (
                  <TableRow>
                    <DefinitionTableCell>Last error</DefinitionTableCell>
                    <DefinitionTableCell>
                      <Typography
                        component="pre"
                        sx={{
                          margin: 0,
                          whiteSpace: 'pre-wrap',
                          fontFamily: 'monospace',
                          fontSize: '0.8rem',
                          color: 'error.main',
                        }}
                      >
                        {event.lastError}
                      </Typography>
                    </DefinitionTableCell>
                  </TableRow>
                )}
                <TableRow>
                  <DefinitionTableCell>Status</DefinitionTableCell>
                  <DefinitionTableCell>
                    {event.status ? (
                      <Box
                        component="pre"
                        sx={{
                          margin: 0,
                          whiteSpace: 'pre-wrap',
                          fontFamily: 'monospace',
                          fontSize: '0.8rem',
                        }}
                      >
                        {event.status}
                      </Box>
                    ) : (
                      '—'
                    )}
                  </DefinitionTableCell>
                </TableRow>
              </TableBody>
            </Table>
          </TableContainer>
        </Stack>
      </Box>
    </Container>
  )
}
