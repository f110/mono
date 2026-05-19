import * as React from 'react'
import {
  Box,
  Stack,
  Container,
  Link,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Typography,
} from '@mui/material'
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
                    <TableCell>Source Repository</TableCell>
                    <TableCell>Job</TableCell>
                    <TableCell>Watched Repository</TableCell>
                    <TableCell>Kind</TableCell>
                    <TableCell>Tag Pattern</TableCell>
                    <TableCell>Pre-release</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {triggers.map((t) => (
                    <TableRow key={t.id}>
                      <TableCell>
                        {t.repositoryUrl ? (
                          <Link href={t.repositoryUrl}>{t.repositoryName}</Link>
                        ) : (
                          t.repositoryName
                        )}
                      </TableCell>
                      <TableCell>{t.jobName}</TableCell>
                      <TableCell>
                        {t.externalRepoUrl ? (
                          <Link href={t.externalRepoUrl}>{t.externalRepo}</Link>
                        ) : (
                          t.externalRepo
                        )}
                      </TableCell>
                      <TableCell>
                        <Chip label={t.kind || 'release'} size="small" />
                      </TableCell>
                      <TableCell>
                        <code>{t.tagPattern || '*'}</code>
                      </TableCell>
                      <TableCell>{t.includePrerelease ? 'yes' : 'no'}</TableCell>
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
