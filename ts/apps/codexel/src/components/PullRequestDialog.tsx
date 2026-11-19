import { useState } from 'react'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Link, MenuItem, Stack, TextField, Typography } from '@mui/material'
import { PullRequestAction, PullRequestService, type Commit } from '../connect/codexel_pb.js'

const actionLabel: Record<PullRequestAction, string> = {
  [PullRequestAction.UNSPECIFIED]: '',
  [PullRequestAction.CREATED]: 'Created',
  [PullRequestAction.UPDATED]: 'Updated',
  [PullRequestAction.EXISTING]: 'Already exists',
}

// noTemplate is the value of the selection not to use the template.
const noTemplate = '\0'

// PullRequestDialog creates the pull requests of the revisions like jj pr submit. One revision submits the stack from
// trunk to it, and multiple revisions are submitted as one pull request. The template has to be picked if there are
// multiple templates like jj pr. onSubmitted is called after the commits are pushed.
export const PullRequestDialog = ({
  repoPath,
  commits,
  onSubmitted,
  onClose,
}: {
  repoPath: string
  commits: Commit[]
  onSubmitted: () => void
  onClose: () => void
}) => {
  const revisions = commits.map((c) => c.commitId)
  const validation = useQuery(PullRequestService.method.validate, { path: repoPath, revisions }, { gcTime: 0 })
  const { data: templateData } = useQuery(PullRequestService.method.listTemplates, { path: repoPath }, { gcTime: 0 })
  const submit = useMutation(PullRequestService.method.submit)
  const [template, setTemplate] = useState<string>()
  const templates = templateData?.templates ?? []
  const reason = validation.data?.reason
  const needsTemplate = templates.length > 1 && template === undefined
  const ready = validation.isSuccess && reason === '' && templateData !== undefined && !needsTemplate

  const run = () => {
    submit.mutate(
      { path: repoPath, revisions, template: template === undefined || template === noTemplate ? '' : template },
      { onSuccess: () => onSubmitted() },
    )
  }

  return (
    <Dialog open onClose={submit.isPending ? undefined : onClose} fullWidth maxWidth="sm">
      <DialogTitle>{commits.length > 1 ? 'Create one pull request of the revisions' : 'Create the pull requests of the stack'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <Typography variant="body2" color="text.secondary">
            {commits.length > 1
              ? 'The same as jj pr submit --single-pr. The revisions must be all commits from trunk.'
              : 'The same as jj pr submit -r. The pull requests are created or updated for each commit from trunk to the revision.'}
          </Typography>
          <Box>
            {commits.map((c) => (
              <Box key={c.commitId} sx={{ fontFamily: 'monospace', fontSize: 13 }}>
                {c.changeId.slice(0, 8)} {c.description || '(no description set)'}
              </Box>
            ))}
          </Box>
          {validation.error && <Alert severity="error">{validation.error.message}</Alert>}
          {reason && <Alert severity="warning">{reason}</Alert>}
          {templates.length === 1 && <Typography variant="body2">Template: {templates[0]}</Typography>}
          {templates.length > 1 && (
            <TextField
              select
              label="Template"
              value={template ?? ''}
              onChange={(e) => setTemplate(e.target.value)}
              disabled={submit.isPending || submit.isSuccess}
              helperText="The template of the new pull requests"
            >
              {templates.map((t) => (
                <MenuItem key={t} value={t}>
                  {t}
                </MenuItem>
              ))}
              <MenuItem value={noTemplate}>Don't use</MenuItem>
            </TextField>
          )}
          {submit.error && <Alert severity="error" sx={{ whiteSpace: 'pre-wrap' }}>{submit.error.message}</Alert>}
          {submit.data && (
            <Box>
              {submit.data.pushed.map((id) => (
                <Typography key={id} variant="body2" sx={{ fontFamily: 'monospace' }}>
                  Pushed {id.slice(0, 12)}
                </Typography>
              ))}
              {submit.data.pullRequests.map((pr, i) => (
                <Typography key={i} variant="body2" sx={{ fontFamily: 'monospace' }}>
                  {actionLabel[pr.action]} {pr.changeId.slice(0, 12)}{' '}
                  {pr.url && (
                    <Link href={pr.url} target="_blank" rel="noreferrer">
                      {pr.url}
                    </Link>
                  )}
                </Typography>
              ))}
              {submit.data.pullRequests.length === 0 && <Typography variant="body2">There are no pull requests to create or update</Typography>}
            </Box>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={submit.isPending}>
          {submit.isSuccess ? 'Close' : 'Cancel'}
        </Button>
        {!submit.isSuccess && (
          <Button variant="contained" onClick={run} disabled={!ready || submit.isPending}>
            {submit.isPending ? 'Submitting…' : 'Submit'}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  )
}
