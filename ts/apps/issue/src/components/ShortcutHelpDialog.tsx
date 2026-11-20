import { useRouterState } from '@tanstack/react-router'
import { Dialog, DialogContent, DialogTitle, Table, TableBody, TableCell, TableRow, Typography } from '@mui/material'
import { type ShortcutHelp, globalShortcuts, inputShortcuts, listShortcuts, ticketShortcuts } from '../shortcuts.ts'

const screens: { routeId: string; title: string; shortcuts: ShortcutHelp[] }[] = [
  { routeId: '/', title: '一覧', shortcuts: Object.values(listShortcuts) },
  { routeId: '/$number', title: '詳細', shortcuts: Object.values(ticketShortcuts) },
]

// ShortcutHelpDialog shows the shortcuts of all screens and the current screen.
export const ShortcutHelpDialog = ({ open, onClose }: { open: boolean; onClose: () => void }) => {
  const routeId = useRouterState({ select: (s) => s.matches.at(-1)?.routeId })
  const sections = [
    { title: '全画面', shortcuts: [...Object.values(globalShortcuts), ...inputShortcuts] },
    ...screens.filter((s) => s.routeId === routeId),
  ]

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>キーボードショートカット</DialogTitle>
      <DialogContent>
        {sections.map((section) => (
          <Table key={section.title} size="small" sx={{ mb: 2 }}>
            <TableBody>
              <TableRow>
                <TableCell colSpan={2}>
                  <Typography variant="subtitle2">{section.title}</Typography>
                </TableCell>
              </TableRow>
              {section.shortcuts.map((s) => (
                <TableRow key={s.label}>
                  <TableCell sx={{ fontFamily: 'monospace', whiteSpace: 'nowrap' }}>{s.label}</TableCell>
                  <TableCell>{s.description}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ))}
      </DialogContent>
    </Dialog>
  )
}
