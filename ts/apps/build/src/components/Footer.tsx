import Box from '@mui/material/Box'
import Typography from '@mui/material/Typography'
import dayjs from 'dayjs'
import { useGetServerInfo } from '../hooks/useGetServerInfo'

export const Footer: React.FC = () => {
  const { commitSha, commitTime } = useGetServerInfo()
  if (!commitSha) {
    return null
  }

  return (
    <Box component="footer" sx={{ mt: 3, textAlign: 'center' }}>
      <Typography variant="caption" color="text.secondary">
        {commitSha}
        {commitTime && ` (${dayjs(commitTime).format('YYYY-MM-DD HH:mm:ss')})`}
      </Typography>
    </Box>
  )
}
