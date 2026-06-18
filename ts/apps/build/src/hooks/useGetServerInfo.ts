import { create } from '@bufbuild/protobuf'
import { useQuery } from '@connectrpc/connect-query'
import { BFF } from '../connect/bff_pb'

export interface ServerConfig {
  dev: boolean
  leaderElection: boolean
  namespace: string
  useBazelisk: boolean
  defaultBazelVersion: string
  remoteCache: string
  taskCpuLimit: string
  taskMemoryLimit: string
  gcEnabled: boolean
  gitDataServiceListen: string
  gitDataServiceUrl: string
  gitDataRefreshInterval: string
  gitDataRefreshWorkers: number
  externalReleasePollInterval: string
  eventReconcileInterval: string
  githubAppId: number
  vaultAddr: string
  dashboardUrl: string
}

export interface ServerInfo {
  supportedBazelVersions: string[]
  schemaVersion: string
  config?: ServerConfig
}

export function useGetServerInfo(): ServerInfo {
  const res = useQuery(
    BFF.method.getServerInfo,
    create(BFF.method.getServerInfo.input),
  )
  if (!res.data) {
    return { supportedBazelVersions: [], schemaVersion: '' }
  }
  const c = res.data.config
  return {
    supportedBazelVersions: res.data.supportedBazelVersions,
    schemaVersion: res.data.schemaVersion,
    config: c
      ? {
          dev: c.dev,
          leaderElection: c.leaderElection,
          namespace: c.namespace,
          useBazelisk: c.useBazelisk,
          defaultBazelVersion: c.defaultBazelVersion,
          remoteCache: c.remoteCache,
          taskCpuLimit: c.taskCpuLimit,
          taskMemoryLimit: c.taskMemoryLimit,
          gcEnabled: c.gcEnabled,
          gitDataServiceListen: c.gitDataServiceListen,
          gitDataServiceUrl: c.gitDataServiceUrl,
          gitDataRefreshInterval: c.gitDataRefreshInterval,
          gitDataRefreshWorkers: c.gitDataRefreshWorkers,
          externalReleasePollInterval: c.externalReleasePollInterval,
          eventReconcileInterval: c.eventReconcileInterval,
          githubAppId: Number(c.githubAppId),
          vaultAddr: c.vaultAddr,
          dashboardUrl: c.dashboardUrl,
        }
      : undefined,
  }
}
