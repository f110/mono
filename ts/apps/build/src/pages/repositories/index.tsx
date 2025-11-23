import { create } from '@bufbuild/protobuf'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { useState } from 'react'
import * as React from 'react'
import {
  Box,
  Button,
  Stack,
  List,
  ListItem,
  ListItemText,
  Link,
  ListItemAvatar,
  Avatar,
  IconButton,
  Container,
  Badge,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import SourceIcon from '@mui/icons-material/Source'
import LockIcon from '@mui/icons-material/Lock'
import SyncIcon from '@mui/icons-material/Sync'
import { useForm } from 'react-hook-form'
import { NewRepositoryModal } from '../../components/NewRepositoryModal.tsx'
import { BFF } from '../../connect/bff_pb'
import { useRemoveRepository } from '../../hooks/useDeleteRepository.ts'
import { useNewRepository } from '../../hooks/useNewRepository.ts'
import { useRepositoryDeleteConfirmDialog } from '../../hooks/useRepositoryDeleteConfirmDialog.tsx'
import type { Repository } from '../../model/msg_pb'

interface NewRepositoryForm {
  name: string
  url: string
  clone_url: string
  is_private: boolean
}

export const RepositoriesPage: React.FC = () => {
  const [newModal, setNewModal] = useState<boolean>(false)
  const handleNewModalOpen = () => setNewModal(true)
  const handleNewModalClose = () => setNewModal(false)

  const {
    register,
    handleSubmit,
    reset: resetForm,
  } = useForm<NewRepositoryForm>()
  const { mutate: saveRepository } = useNewRepository()
  const handleNew = (data: NewRepositoryForm) => {
    saveRepository({
      repository: {
        name: data.name,
        url: data.url,
        cloneUrl: data.clone_url,
        private: data.is_private,
      },
    })
    setNewModal(false)
    resetForm()
  }
  const { data: repositories } = useQuery(
    BFF.method.listRepositories,
    create(BFF.method.listRepositories.input),
  )

  const { ConfirmDialog, openConfirmDialog } =
    useRepositoryDeleteConfirmDialog()
  const { mutate: removeRepository } = useRemoveRepository()
  const openDialog = async (repository: Repository) => {
    const result = await openConfirmDialog(repository)
    removeRepository({
      repositoryId: result,
    })
  }
  const { mutate: syncRepository } = useMutation(BFF.method.syncRepository)
  const onSyncRepository = (id: number) => {
    syncRepository({ repositoryId: id })
  }

  return (
    <Container>
      <Box sx={{ width: '100%' }}>
        <Stack spacing={2}>
          <Stack direction="row">
            <Button
              variant="contained"
              color="primary"
              startIcon={<AddIcon />}
              onClick={handleNewModalOpen}
            >
              New
            </Button>
          </Stack>

          <List>
            {repositories?.repositories.map((repository) => (
              <ListItem
                key={repository.name}
                secondaryAction={
                  <Stack direction="row">
                    <IconButton
                      aria-label="sync"
                      onClick={() => onSyncRepository(repository.id)}
                    >
                      <SyncIcon color="primary" />
                    </IconButton>
                    <IconButton
                      edge="end"
                      aria-label="delete"
                      onClick={() => openDialog(repository)}
                    >
                      <DeleteIcon />
                    </IconButton>
                  </Stack>
                }
              >
                <ListItemAvatar>
                  {repository.private ? (
                    <Badge
                      overlap="circular"
                      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                      badgeContent={<LockIcon color="inherit" />}
                    >
                      <Avatar>
                        <SourceIcon />
                      </Avatar>
                    </Badge>
                  ) : (
                    <Avatar>
                      <SourceIcon />
                    </Avatar>
                  )}
                </ListItemAvatar>
                <ListItemText
                  primary={repository.name}
                  secondary={
                    <Link href={repository.url}>{repository.url}</Link>
                  }
                />
              </ListItem>
            ))}
          </List>
        </Stack>
      </Box>

      <NewRepositoryModal
        form={register}
        open={newModal}
        onClose={handleNewModalClose}
        onSubmit={handleSubmit(handleNew)}
      />

      <ConfirmDialog />
    </Container>
  )
}
