import * as React from 'react'
import {
  Box,
  Stack,
  Container,
  Link,
  Chip,
  Table,
  TableBody,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Typography,
} from '@mui/material'
import { StyledTableCell, StyledTableRow } from '../../components/Table.tsx'
import { useListExternalReleaseTriggers } from '../../hooks/useListExternalReleaseTriggers.ts'

export const ExternalReleaseTriggersPage: React.FC = () => {
  const triggers = useListExternalReleaseTriggers()

  return (
    <Container>
      <Box sx={{ width: '100%' }}>
        <Stack spacing={2}>
          <Typography variant="h5">External Release Triggers</Typography>
          {triggers.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No external release triggers configured.
            </Typography>
          ) : (
            <TableContainer component={Paper}>
              <Table size="small" aria-label="external release triggers">
                <TableHead>
                  <TableRow>
                    <StyledTableCell>Source Repository</StyledTableCell>
                    <StyledTableCell>Job</StyledTableCell>
                    <StyledTableCell>Watched Repository</StyledTableCell>
                    <StyledTableCell>Kind</StyledTableCell>
                    <StyledTableCell>Tag Pattern</StyledTableCell>
                    <StyledTableCell>Pre-release</StyledTableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {triggers.map((t) => (
                    <StyledTableRow key={t.id}>
                      <StyledTableCell>
                        {t.repositoryUrl ? (
                          <Link href={t.repositoryUrl}>{t.repositoryName}</Link>
                        ) : (
                          t.repositoryName
                        )}
                      </StyledTableCell>
                      <StyledTableCell>{t.jobName}</StyledTableCell>
                      <StyledTableCell>
                        {t.externalRepoUrl ? (
                          <Link href={t.externalRepoUrl}>{t.externalRepo}</Link>
                        ) : (
                          t.externalRepo
                        )}
                      </StyledTableCell>
                      <StyledTableCell>
                        <Chip label={t.kind || 'release'} size="small" />
                      </StyledTableCell>
                      <StyledTableCell>
                        <code>{t.tagPattern || '*'}</code>
                      </StyledTableCell>
                      <StyledTableCell>
                        {t.includePrerelease ? 'yes' : 'no'}
                      </StyledTableCell>
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
