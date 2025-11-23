import {
  Box,
  Button,
  FormControlLabel,
  FormGroup,
  Modal,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material'
import Container from '@mui/material/Container'
import * as React from 'react'
import type { UseFormRegister } from 'react-hook-form'

const modalStyle = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  bgcolor: 'background.paper',
  border: '0',
  boxShadow: 24,
  p: 2,
}

type Props = {
  form: UseFormRegister<any>
  open: boolean
  onClose: () => void
  onSubmit: () => void
}

export const NewRepositoryModal: React.FC<Props> = ({
  form,
  open,
  onClose,
  onSubmit,
}) => {
  return (
    <Modal
      open={open}
      onClose={onClose}
      aria-labelledby="modal-modal-title"
      aria-describedby="modal-modal-description"
    >
      <Container sx={modalStyle} maxWidth="lg">
        <Box sx={{ width: '100%' }}>
          <Stack spacing={2}>
            <Typography variant="h6">New Repository</Typography>
            <Stack spacing={1}>
              <TextField label="Name" {...form('name')}></TextField>
              <TextField label="Url" {...form('url')}></TextField>
              <TextField label="Clone URL" {...form('clone_url')}></TextField>
              <FormGroup>
                <FormControlLabel
                  control={<Switch />}
                  label="Private Repository"
                  {...form('is_private')}
                ></FormControlLabel>
              </FormGroup>
              <Box>
                <Button
                  variant="contained"
                  color="primary"
                  onClick={onSubmit}
                  sx={{ textTransform: 'none' }}
                >
                  Add
                </Button>
              </Box>
            </Stack>
          </Stack>
        </Box>
      </Container>
    </Modal>
  )
}
