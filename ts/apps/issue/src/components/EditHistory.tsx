import { useState } from 'react'
import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import { skipToken, useQuery } from '@connectrpc/connect-query'
import { Box, Button, Dialog, DialogContent, DialogTitle, Link, Menu, MenuItem, Typography } from '@mui/material'
import dayjs from 'dayjs'
import { diffLines } from 'diff'
import { Issue } from '../connect/issue_pb'
import { useMasters } from '../hooks/useMasters.ts'

type Edit = {
  actorId: number
  createdAt?: Timestamp
  created: boolean
  before: string
  after: string
}

// EditHistory is the "（編集済み）" link which lists the edits like GitHub. Selecting the edit shows the diff of it.
// The revisions are fetched when the list is opened.
export const EditHistory = ({
  target,
  current,
  authorId,
  createdAt,
}: {
  target: { ticket: number } | { comment: number }
  current: string
  authorId: number
  createdAt?: Timestamp
}) => {
  const { actorName } = useMasters()
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const [selected, setSelected] = useState<Edit | null>(null)
  const open = anchor !== null || selected !== null
  const ticketRevisions = useQuery(Issue.method.listTicketRevisions, open && 'ticket' in target ? { ticket: target.ticket } : skipToken)
  const commentRevisions = useQuery(Issue.method.listCommentRevisions, open && 'comment' in target ? { id: target.comment } : skipToken)
  const revisions = ('ticket' in target ? ticketRevisions : commentRevisions).data?.revisions ?? []

  const edits: Edit[] = revisions.map((r, i) => ({
    actorId: r.actorId,
    createdAt: r.createdAt,
    created: false,
    before: r.body,
    after: i + 1 < revisions.length ? revisions[i + 1].body : current,
  }))
  if (revisions.length > 0) {
    edits.unshift({ actorId: authorId, createdAt, created: true, before: '', after: revisions[0].body })
  }
  const describe = (e: Edit) => `${actorName(e.actorId)} が${e.created ? '作成' : '編集'} ${e.createdAt ? dayjs(timestampDate(e.createdAt)).fromNow() : ''}`

  return (
    <>
      <Link component="button" variant="body2" color="inherit" underline="hover" sx={{ verticalAlign: 'baseline' }} onClick={(e) => setAnchor(e.currentTarget)}>
        （編集済み）
      </Link>
      <Menu anchorEl={anchor} open={anchor !== null} onClose={() => setAnchor(null)}>
        {edits.length === 0 && <MenuItem disabled>読み込み中</MenuItem>}
        {[...edits].reverse().map((e) => (
          <MenuItem
            key={`${e.created}-${e.createdAt?.seconds}-${e.actorId}`}
            onClick={() => {
              setAnchor(null)
              setSelected(e)
            }}
          >
            {describe(e)}
          </MenuItem>
        ))}
      </Menu>
      <Dialog open={selected !== null} onClose={() => setSelected(null)} maxWidth="md" fullWidth>
        {selected && (
          <>
            <DialogTitle>{describe(selected)}</DialogTitle>
            <DialogContent>
              <Diff before={selected.before} after={selected.after} />
            </DialogContent>
          </>
        )}
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', px: 3, pb: 2 }}>
          <Button onClick={() => setSelected(null)}>閉じる</Button>
        </Box>
      </Dialog>
    </>
  )
}

const Diff = ({ before, after }: { before: string; after: string }) => (
  <Box
    component="pre"
    sx={{ m: 0, p: 1, border: 1, borderColor: 'divider', borderRadius: 1, overflowX: 'auto', fontFamily: 'monospace', fontSize: '0.8125rem' }}
  >
    {diffLines(before, after).flatMap((change, i) =>
      change.value
        .replace(/\n$/, '')
        .split('\n')
        .map((line, j) => (
          <Typography
            key={`${i}-${j}`}
            component="div"
            sx={{
              font: 'inherit',
              whiteSpace: 'pre-wrap',
              px: 1,
              backgroundColor: change.added ? '#e6ffec' : change.removed ? '#ffebe9' : undefined,
            }}
          >
            {change.added ? '+ ' : change.removed ? '- ' : '  '}
            {line}
          </Typography>
        )),
    )}
  </Box>
)
