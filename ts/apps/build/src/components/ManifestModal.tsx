import * as React from 'react'
import { Modal, Container, Typography, Box, Stack, Paper } from '@mui/material'

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
  manifest: string
}

export const ManifestModal: React.FC<Props> = ({ open, onClose, manifest }) => {
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
            <Typography variant="h6">Job manifest</Typography>
            <Box sx={{ overflow: 'auto' }}>
              <Paper sx={{ fontFamily: 'monospace' }}>
                <pre>
                  <code>{manifest}</code>
                </pre>
              </Paper>
            </Box>
          </Stack>
        </Box>
      </Container>
    </Modal>
  )
}
