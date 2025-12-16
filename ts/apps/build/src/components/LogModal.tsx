import { useQuery } from '@connectrpc/connect-query'
import * as React from 'react'
import { Modal, Container, Typography, Box, Stack, Paper } from '@mui/material'
import { BFF } from '../connect/bff_pb'

const modalStyle = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  bgcolor: 'background.paper',
  border: '0',
  boxShadow: 24,
  p: 2,
  height: '80%',
}

type Props = {
  open: boolean
  onClose: () => void
  taskId: number
}

export const LogModal: React.FC<Props> = ({ open, onClose, taskId }) => {
  if (!open) {
    return null
  }

  const { data: logs, isSuccess } = useQuery(BFF.method.getLogs, {
    taskId: taskId,
  })
  if (!isSuccess) {
    return null
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      aria-labelledby="modal-modal-title"
      aria-describedby="modal-modal-description"
    >
      <Container sx={modalStyle} maxWidth="lg">
        <Box sx={{ width: '100%', height: '100%' }}>
          <Stack spacing={2} sx={{ height: '100%' }}>
            <Typography variant="h6">Log</Typography>
            <Box sx={{ overflow: 'auto' }}>
              <Paper sx={{ fontFamily: 'monospace' }}>
                <pre>{logs?.body}</pre>
              </Paper>
            </Box>
          </Stack>
        </Box>
      </Container>
    </Modal>
  )
}
