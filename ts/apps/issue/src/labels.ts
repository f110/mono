import { ActivityKind, ActorKind, Priority, RelationType, TicketStatus } from './connect/issue_pb'

export const statusLabels: Record<TicketStatus, string> = {
  [TicketStatus.UNSPECIFIED]: '-',
  [TicketStatus.OPEN]: 'Open',
  [TicketStatus.IN_PROGRESS]: 'In Progress',
  [TicketStatus.DONE]: 'Done',
  [TicketStatus.WONT_FIX]: "Won't Fix",
}

export const statusColors: Record<TicketStatus, { backgroundColor?: string; color?: string }> = {
  [TicketStatus.UNSPECIFIED]: {},
  [TicketStatus.OPEN]: { backgroundColor: '#1976d2', color: '#fff' },
  [TicketStatus.IN_PROGRESS]: { backgroundColor: '#2e7d32', color: '#fff' },
  [TicketStatus.DONE]: { backgroundColor: '#8250df', color: '#fff' },
  [TicketStatus.WONT_FIX]: {},
}

export const statuses = [TicketStatus.OPEN, TicketStatus.IN_PROGRESS, TicketStatus.DONE, TicketStatus.WONT_FIX]

export const priorityLabels: Record<Priority, string> = {
  [Priority.UNSPECIFIED]: 'なし',
  [Priority.P0]: 'P0',
  [Priority.P1]: 'P1',
  [Priority.P2]: 'P2',
  [Priority.P3]: 'P3',
  [Priority.P4]: 'P4',
}

export const priorities = [Priority.UNSPECIFIED, Priority.P0, Priority.P1, Priority.P2, Priority.P3, Priority.P4]

export const relationLabels: Record<RelationType, [string, string]> = {
  [RelationType.UNSPECIFIED]: ['-', '-'],
  [RelationType.PARENT_OF]: ['Parent of', 'Child of'],
  [RelationType.BLOCKS]: ['Blocks', 'Blocked by'],
  [RelationType.DUPLICATE_OF]: ['Duplicate of', 'Duplicated by'],
  [RelationType.RELATES_TO]: ['Relates to', 'Relates to'],
}

export const relationTypes = [RelationType.PARENT_OF, RelationType.BLOCKS, RelationType.DUPLICATE_OF, RelationType.RELATES_TO]

export const actorKindLabels: Record<ActorKind, string> = {
  [ActorKind.UNSPECIFIED]: '',
  [ActorKind.HUMAN]: '',
  [ActorKind.AGENT]: 'エージェント',
  [ActorKind.SYSTEM]: 'システム',
}

export const activityLabels: Record<ActivityKind, string> = {
  [ActivityKind.UNSPECIFIED]: '',
  [ActivityKind.CREATE]: 'がチケットを作成',
  [ActivityKind.TITLE]: 'がタイトルを変更',
  [ActivityKind.STATUS]: 'がステータスを変更',
  [ActivityKind.PRIORITY]: 'が優先度を変更',
  [ActivityKind.COMPONENT]: 'が分類を変更',
  [ActivityKind.ASSIGNEE]: 'が担当を変更',
  [ActivityKind.BODY]: 'が本文を編集',
  [ActivityKind.RELATION_ADDED]: 'が関連を追加',
  [ActivityKind.RELATION_REMOVED]: 'が関連を削除',
  [ActivityKind.COMMENT]: 'がコメント',
  [ActivityKind.COMMENT_EDITED]: 'がコメントを編集',
  [ActivityKind.COMMENT_DELETED]: 'がコメントを削除',
  [ActivityKind.COMMIT_LINKED]: 'がコミットを関連付け',
  [ActivityKind.PULL_REQUEST_LINKED]: 'が Pull Request を関連付け',
  [ActivityKind.MENTIONED]: 'がこのチケットに言及',
  [ActivityKind.MENTION_REMOVED]: 'がこのチケットへの言及を削除',
}
