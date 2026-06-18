import {
  Container,
  TableContainer,
  Table,
  TableRow,
  TableCell,
  TableBody,
  Box,
  List,
  ListItem,
  Typography,
} from '@mui/material'
import { styled } from '@mui/material/styles'
import * as React from 'react'
import {
  useGetServerInfo,
  type ServerConfig,
} from '../../hooks/useGetServerInfo.ts'

const DefinitionTableCell = styled(TableCell)(({ theme }) => ({
  '&:first-child': {
    backgroundColor: theme.palette.action.hover,
  },
}))

// configItems interprets the raw configuration values into human-meaningful
// settings rather than exposing them verbatim. Secrets are never included.
function configItems(c: ServerConfig): { label: string; value: string }[] {
  return [
    {
      label: 'Kubernetes integration',
      value: c.dev ? 'Disabled (development mode)' : 'Enabled',
    },
    {
      label: 'High availability (leader election)',
      value: c.leaderElection ? 'Enabled' : 'Disabled (single instance)',
    },
    { label: 'Job namespace', value: c.namespace || '(default)' },
    {
      label: 'Bazel version management',
      value: c.useBazelisk
        ? 'Bazelisk'
        : `Fixed (${c.defaultBazelVersion || 'unset'})`,
    },
    {
      label: 'Remote cache',
      value: c.remoteCache || 'Disabled',
    },
    {
      label: 'Job default resource limits',
      value: `CPU ${c.taskCpuLimit || 'unset'} / Memory ${c.taskMemoryLimit || 'unset'}`,
    },
    { label: 'Log GC', value: c.gcEnabled ? 'Enabled' : 'Disabled' },
    {
      label: 'Embedded git-data-service',
      value: c.gitDataServiceListen || 'Disabled',
    },
    {
      label: 'Repository data source',
      value: c.gitDataServiceUrl
        ? `git-data-service (${c.gitDataServiceUrl})`
        : 'GitHub (direct)',
    },
    {
      label: 'Repository refresh',
      value: c.gitDataRefreshInterval
        ? `Every ${c.gitDataRefreshInterval} (workers: ${c.gitDataRefreshWorkers})`
        : 'Disabled',
    },
    {
      label: 'GitHub event reconcile interval',
      value: c.eventReconcileInterval || 'unset',
    },
    {
      label: 'External release poll interval',
      value: c.externalReleasePollInterval || 'unset',
    },
    {
      label: 'GitHub App',
      value: c.githubAppId > 0 ? `Configured (App ID ${c.githubAppId})` : 'Not configured',
    },
    { label: 'Vault integration', value: c.vaultAddr || 'Disabled' },
    { label: 'Dashboard URL', value: c.dashboardUrl || 'unset' },
  ]
}

export const InfoPage: React.FC = () => {
  const { supportedBazelVersions, schemaVersion, config } = useGetServerInfo()

  return (
    <Container maxWidth="xl">
      <Box sx={{ width: '100%' }}>
        <TableContainer>
          <Table>
            <TableBody>
              <TableRow>
                <DefinitionTableCell>
                  Supported Bazel versions
                </DefinitionTableCell>
                <DefinitionTableCell>
                  <List
                    sx={{
                      listStyleType: 'disc',
                      columnCount: 3,
                      pl: 2,
                      '& .MuiListItem-root': {
                        display: 'list-item',
                        padding: 0,
                      },
                    }}
                  >
                    {supportedBazelVersions.map((version) => (
                      <ListItem>{version}</ListItem>
                    ))}
                  </List>
                </DefinitionTableCell>
              </TableRow>
              <TableRow>
                <DefinitionTableCell>Builder</DefinitionTableCell>
                <DefinitionTableCell>Running</DefinitionTableCell>
              </TableRow>
              <TableRow>
                <DefinitionTableCell>Schema version</DefinitionTableCell>
                <DefinitionTableCell sx={{ fontFamily: 'monospace' }}>
                  {schemaVersion}
                </DefinitionTableCell>
              </TableRow>
            </TableBody>
          </Table>
        </TableContainer>

        {config && (
          <>
            <Typography variant="h6" sx={{ mt: 4, mb: 1 }}>
              Server configuration
            </Typography>
            <TableContainer>
              <Table>
                <TableBody>
                  {configItems(config).map((item) => (
                    <TableRow>
                      <DefinitionTableCell>{item.label}</DefinitionTableCell>
                      <DefinitionTableCell>{item.value}</DefinitionTableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </>
        )}
      </Box>
    </Container>
  )
}
