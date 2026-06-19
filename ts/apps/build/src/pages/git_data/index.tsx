import {
  Box,
  Collapse,
  Container,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material'
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown'
import KeyboardArrowUpIcon from '@mui/icons-material/KeyboardArrowUp'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import dayjs from 'dayjs'
import * as React from 'react'
import type { GitDataRepository } from '../../connect/bff_pb'
import { useListGitData } from '../../hooks/useListGitData.ts'
import { useGetGitDataStatistics } from '../../hooks/useGetGitDataStatistics.ts'

const RepositoryRow: React.FC<{ repository: GitDataRepository }> = ({
  repository,
}) => {
  const [open, setOpen] = React.useState(false)
  const { data, isLoading } = useGetGitDataStatistics(repository.name, open)

  const lastUpdate = data?.headCommitWhen
    ? dayjs(timestampDate(data.headCommitWhen)).format('YYYY-MM-DD HH:mm:ss')
    : ''

  return (
    <>
      <TableRow sx={{ '& > *': { borderBottom: 'unset' } }}>
        <TableCell>
          <IconButton size="small" onClick={() => setOpen(!open)}>
            {open ? <KeyboardArrowUpIcon /> : <KeyboardArrowDownIcon />}
          </IconButton>
        </TableCell>
        <TableCell>{repository.name}</TableCell>
        <TableCell sx={{ fontFamily: 'monospace' }}>
          {repository.defaultBranch}
        </TableCell>
        <TableCell>
          {repository.url ? (
            <a href={repository.url} target="_blank" rel="noreferrer">
              {repository.url}
            </a>
          ) : (
            ''
          )}
        </TableCell>
      </TableRow>
      <TableRow>
        <TableCell sx={{ py: 0 }} colSpan={4}>
          <Collapse in={open} timeout="auto" unmountOnExit>
            <Box sx={{ m: 2 }}>
              {isLoading && <Typography>Loading...</Typography>}
              {data && (
                <Table size="small">
                  <TableBody>
                    <TableRow>
                      <TableCell variant="head">Commit count</TableCell>
                      <TableCell>{data.commitCount.toString()}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell variant="head">Last update</TableCell>
                      <TableCell>{lastUpdate}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell variant="head">HEAD commit</TableCell>
                      <TableCell sx={{ fontFamily: 'monospace' }}>
                        {data.headCommitSha}
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell variant="head">Author</TableCell>
                      <TableCell>{data.headCommitAuthor}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell variant="head">Message</TableCell>
                      <TableCell sx={{ whiteSpace: 'pre-wrap' }}>
                        {data.headCommitMessage}
                      </TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              )}
            </Box>
          </Collapse>
        </TableCell>
      </TableRow>
    </>
  )
}

export const GitDataPage: React.FC = () => {
  const repositories = useListGitData()

  return (
    <Container maxWidth="xl">
      <Box sx={{ width: '100%' }}>
        <Typography variant="h6" sx={{ mb: 1 }}>
          Git Data
        </Typography>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell />
                <TableCell>Name</TableCell>
                <TableCell>Default branch</TableCell>
                <TableCell>URL</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {repositories.map((repository) => (
                <RepositoryRow key={repository.name} repository={repository} />
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Container>
  )
}
