import {
  Container,
  TableContainer,
  Table,
  TableRow,
  TableCell,
  TableBody,
  Box,
  List,
  ListItem,
} from '@mui/material'
import { styled } from '@mui/material/styles'
import * as React from 'react'
import { useGetServerInfo } from '../../hooks/useGetServerInfo.ts'

const DefinitionTableCell = styled(TableCell)(({ theme }) => ({
  '&:first-child': {
    backgroundColor: theme.palette.action.hover,
  },
}))

export const InfoPage: React.FC = () => {
  const supportedVersions = useGetServerInfo()

  return (
    <Container maxWidth="xl">
      <Box sx={{ width: '100%' }}>
        <TableContainer>
          <Table>
            <TableBody>
              <TableRow>
                <DefinitionTableCell>
                  Supported Bazel versions
                </DefinitionTableCell>
                <DefinitionTableCell>
                  <List
                    sx={{
                      listStyleType: 'disc',
                      columnCount: 3,
                      pl: 2,
                      '& .MuiListItem-root': {
                        display: 'list-item',
                        padding: 0,
                      },
                    }}
                  >
                    {supportedVersions.map((version) => (
                      <ListItem>{version}</ListItem>
                    ))}
                  </List>
                </DefinitionTableCell>
              </TableRow>
              <TableRow>
                <DefinitionTableCell>Builder</DefinitionTableCell>
                <DefinitionTableCell>Running</DefinitionTableCell>
              </TableRow>
              <TableRow>
                <DefinitionTableCell>Schema version</DefinitionTableCell>
                <DefinitionTableCell></DefinitionTableCell>
              </TableRow>
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Container>
  )
}
