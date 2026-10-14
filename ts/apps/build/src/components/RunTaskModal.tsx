import { useQuery } from '@connectrpc/connect-query'
import { useState } from 'react'
import * as React from 'react'
import {
  Modal,
  Container,
  Typography,
  Box,
  Stack,
  Button,
  InputLabel,
  Select,
  MenuItem,
  FormControl,
  type SelectChangeEvent,
  Alert,
  TextField,
} from '@mui/material'
import { BFF } from '../connect/bff_pb'
import { useInvokeJob } from '../hooks/useInvokeJob.ts'
import type { Job, Repository } from '../model/msg_pb'

type JobSelectProps = {
  repositoryId: number | undefined
  onSelected: (job: Job) => void
}

const JobSelect: React.FC<JobSelectProps> = ({ repositoryId, onSelected }) => {
  if (!repositoryId) {
    return (
      <Select labelId="work" label="Task" value="" disabled={true}></Select>
    )
  }

  const { data } = useQuery(BFF.method.listJobs, { repositoryId: repositoryId })
  const [job, setJob] = useState<Job | undefined>(undefined)
  const onSelect = (select: SelectChangeEvent) => {
    const selected = data?.jobs.find((job) => job.name === select.target.value)
    if (!selected) {
      return
    }
    setJob(selected)
    onSelected(selected)
  }

  return (
    <Select
      labelId="work"
      label="Task"
      onChange={onSelect}
      value={job ? job.name : ''}
    >
      {data?.jobs.map((job) => (
        <MenuItem value={job.name}>{job.name}</MenuItem>
      ))}
    </Select>
  )
}

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
  open: boolean
  onClose: () => void
  repositories: Repository[]
}

export const RunTaskModal: React.FC<Props> = ({
  open,
  onClose,
  repositories,
}) => {
  const [repository, setRepository] = useState<Repository | undefined>(
    undefined,
  )
  const onSelect = (select: SelectChangeEvent) => {
    const selectedRepository = repositories.find(
      (r) => r.id === Number(select.target.value),
    )
    setRepository(selectedRepository)
  }
  const [selectedJob, setSelectedJob] = useState<Job | undefined>(undefined)
  const [params, setParams] = useState<Record<string, string>>({})
  const onJobSelected = (job: Job) => {
    setSelectedJob(job)
    setParams({})
  }
  const missingRequiredParam = selectedJob?.params.some(
    (param) => param.required && !params[param.name],
  )
  const [error, setError] = useState<string | undefined>(undefined)
  const { mutate: InvokeJob, isPending } = useInvokeJob()
  const startJob = () => {
    if (!repository || !selectedJob) {
      return
    }

    InvokeJob(
      {
        repositoryId: repository.id,
        jobName: selectedJob.name,
        params: params,
      },
      {
        onSuccess: () => {
          onClose()
        },
        onError: () => {
          setError('Failed to invoke job')
        },
      },
    )
  }

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
            <Typography variant="h6">Run task</Typography>
            {error && <Alert severity="error">{error}</Alert>}
            <Stack spacing={1}>
              <FormControl>
                <InputLabel id="repository">Repository...</InputLabel>
                <Select
                  labelId="repository"
                  label="Repository..."
                  onChange={onSelect}
                  value={repository ? String(repository.id) : ''}
                >
                  {repositories.map((repository) => (
                    <MenuItem value={repository.id}>{repository.name}</MenuItem>
                  ))}
                </Select>
              </FormControl>

              <FormControl>
                <InputLabel id="work">Task</InputLabel>
                <JobSelect
                  repositoryId={repository?.id}
                  onSelected={onJobSelected}
                />
              </FormControl>

              {selectedJob?.params.map((param) => (
                <TextField
                  key={param.name}
                  label={param.name}
                  helperText={param.description}
                  required={param.required}
                  value={params[param.name] ?? ''}
                  onChange={(e) =>
                    setParams({ ...params, [param.name]: e.target.value })
                  }
                />
              ))}

              <Box>
                <Button
                  variant="contained"
                  color="primary"
                  sx={{ textTransform: 'none' }}
                  onClick={startJob}
                  disabled={missingRequiredParam}
                  loading={isPending}
                  loadingPosition="end"
                >
                  Start
                </Button>
              </Box>
            </Stack>
          </Stack>
        </Box>
      </Container>
    </Modal>
  )
}
