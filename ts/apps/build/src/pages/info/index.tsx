import {
  Container,
  TableContainer,
  Table,
  TableRow,
  TableCell,
  TableBody,
  Box,
  Chip,
  Divider,
  Paper,
  Stack,
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

// compareVersion orders two semver strings ascending. The server always sends
// `major.minor.patch`, so a plain component-wise comparison is enough.
function compareVersion(a: string, b: string): number {
  const [pa, pb] = [a.split('.').map(Number), b.split('.').map(Number)]
  for (let i = 0; i < 3; i++) {
    if ((pa[i] ?? 0) !== (pb[i] ?? 0)) return (pa[i] ?? 0) - (pb[i] ?? 0)
  }
  return 0
}

// latestPerMajor picks the newest version of each major series. They are the
// ones highlighted in the version list.
function latestPerMajor(versions: string[]): Set<string> {
  const latest = new Map<number, string>()
  for (const v of versions) {
    const major = Number(v.split('.')[0])
    const current = latest.get(major)
    if (!current || compareVersion(v, current) > 0) latest.set(major, v)
  }
  return new Set(latest.values())
}

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

  const versions = React.useMemo(
    () => [...supportedBazelVersions].sort((a, b) => compareVersion(b, a)),
    [supportedBazelVersions],
  )
  const highlighted = React.useMemo(
    () => latestPerMajor(supportedBazelVersions),
    [supportedBazelVersions],
  )

  return (
    <Container maxWidth="xl">
      <Box sx={{ width: '100%' }}>
        <Paper sx={{ p: 2.5 }}>
          <Stack
            direction="row"
            sx={{ justifyContent: 'space-between', alignItems: 'baseline' }}
          >
            <Typography variant="h6">Supported Bazel versions</Typography>
            <Typography variant="body2" sx={{ color: 'text.secondary' }}>
              {versions.length} versions on the mirror
            </Typography>
          </Stack>
          {config && (
            <Typography variant="body2" sx={{ mt: 0.5, color: 'text.secondary' }}>
              {config.useBazelisk
                ? 'Bazelisk がリポジトリの .bazelversion を解決するため、ミラーに無い版はビルドできません。'
                : `ジョブが版を指定しない場合は ${config.defaultBazelVersion || 'unset'} を使います。`}
            </Typography>
          )}
          <Divider sx={{ my: 1.5 }} />
          {versions.length > 0 ? (
            <Stack direction="row" useFlexGap sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {versions.map((version) => (
                <Chip
                  key={version}
                  size="small"
                  label={version}
                  color={highlighted.has(version) ? 'primary' : 'default'}
                  variant={highlighted.has(version) ? 'filled' : 'outlined'}
                  sx={{
                    fontFamily: 'monospace',
                    fontWeight: highlighted.has(version) ? 600 : 400,
                  }}
                />
              ))}
            </Stack>
          ) : (
            <Typography variant="body2" sx={{ color: 'text.disabled' }}>
              ミラーに Bazel のバイナリが見つかりません。
            </Typography>
          )}
        </Paper>

        <TableContainer sx={{ mt: 3 }}>
          <Table>
            <TableBody>
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
