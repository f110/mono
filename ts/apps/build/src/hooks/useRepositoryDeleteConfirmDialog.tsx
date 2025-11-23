import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material'
import * as React from 'react'
import { useState } from 'react'
import type { Repository } from '../model/msg_pb'

export function useRepositoryDeleteConfirmDialog(): {
  ConfirmDialog: React.FC
  openConfirmDialog: (repository: Repository) => Promise<number>
} {
  const [open, setOpen] = useState<boolean>(false)
  const [repository, setRepository] = useState<Repository | undefined>(
    undefined,
  )
  const [resolve, setResolve] = useState<(id: number) => void>()

  const onClose = () => {
    setOpen(false)
    setRepository(undefined)
  }
  const onConfirmed = () => {
    setOpen(false)
    if (resolve && repository) {
      resolve(repository.id)
    }
  }

  const openConfirmDialog = (repository: Repository): Promise<number> => {
    setRepository(repository)
    setOpen(true)
    return new Promise<number>((resolve) => {
      setResolve(() => resolve)
    })
  }

  const ConfirmDialog: React.FC = () => {
    return (
      repository && (
        <Dialog
          open={open}
          onClose={onClose}
          aria-labelledby="alert-dialog-title"
          aria-describedby="alert-dialog-description"
        >
          <DialogTitle id="alert-dialog-title">
            Are you sure you want to delete the repository?
          </DialogTitle>
          <DialogContent>
            <DialogContentText id="alert-dialog-description">
              {`Are you sure you want to delete the repository "${repository.name}"?`}
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button onClick={onClose}>No</Button>
            <Button onClick={onConfirmed} autoFocus>
              Yes
            </Button>
          </DialogActions>
        </Dialog>
      )
    )
  }

  return {
    ConfirmDialog,
    openConfirmDialog,
  }
}
