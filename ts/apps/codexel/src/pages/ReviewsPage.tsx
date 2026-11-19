import { useQuery } from '@connectrpc/connect-query'
import { Alert, Box, Chip, List, ListItemButton, ListItemText, Stack, Typography } from '@mui/material'
import { useNavigate } from '@tanstack/react-router'
import { ReviewService } from '../connect/codexel_pb.js'
import { stateChip } from '../lib/review.ts'

// ReviewsPage lists the reviews of all repositories.
export const ReviewsPage = () => {
  const { data, error } = useQuery(ReviewService.method.list, { path: '' })
  const navigate = useNavigate()

  if (error) {
    return <Alert severity="error">{error.message}</Alert>
  }
  return (
    <Box sx={{ height: '100%', overflow: 'auto', p: 2 }}>
      {data?.reviews.length === 0 && (
        <Typography color="text.secondary">
          No reviews. Start the review from the selected revisions in the repository (v) or the commits of the agent.
        </Typography>
      )}
      <List dense>
        {data?.reviews.map((r) => (
          <ListItemButton key={r.id} onClick={() => navigate({ to: '/reviews/$reviewId', params: { reviewId: r.id } })}>
            <ListItemText
              primary={
                <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                  <Chip label={stateChip[r.state].label} color={stateChip[r.state].color} size="small" />
                  <Typography>{r.repository.split('/').pop()}</Typography>
                  <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                    {r.changeIds.map((c) => c.slice(0, 8)).join(' ')}
                  </Typography>
                </Stack>
              }
              secondary={`round ${r.rounds.length} · ${r.comments.filter((c) => !c.resolved).length} unresolved · updated ${new Date(Number(r.updatedAt) * 1000).toLocaleString()}`}
            />
          </ListItemButton>
        ))}
      </List>
    </Box>
  )
}
