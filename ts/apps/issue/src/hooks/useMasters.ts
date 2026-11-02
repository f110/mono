import { useQuery } from '@connectrpc/connect-query'
import { type Actor, type Component, ActorKind, Issue } from '../connect/issue_pb'

export function useMasters(): {
  actors: Actor[]
  operators: Actor[]
  components: Component[]
  actorName: (id: number) => string
  componentName: (id: number) => string
} {
  const { data: actors } = useQuery(Issue.method.listActors, {})
  const { data: components } = useQuery(Issue.method.listComponents, {})

  const actorList = actors?.actors ?? []
  const componentList = components?.components ?? []
  return {
    actors: actorList,
    operators: actorList.filter((a) => a.kind !== ActorKind.SYSTEM),
    components: componentList,
    actorName: (id) => actorList.find((a) => a.id === id)?.name ?? '',
    componentName: (id) => componentList.find((c) => c.id === id)?.name ?? '',
  }
}
