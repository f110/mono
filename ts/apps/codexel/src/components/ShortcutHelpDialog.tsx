import { useRouterState } from '@tanstack/react-router'
import { Dialog, DialogContent, DialogTitle, Table, TableBody, TableCell, TableRow, Typography } from '@mui/material'

// ShortcutHelp is the row of the help: the label of the keys and the description of the action.
type ShortcutHelp = [string, string]

type Section = { title: string; shortcuts: ShortcutHelp[] }

const globalSection: Section = {
  title: 'All screens',
  shortcuts: [
    ['?', 'Show the keyboard shortcuts'],
    ['Esc', 'Blur the input (the text is kept)'],
    ['Cmd/Ctrl+Enter', 'Submit the multi-line input'],
  ],
}

const diffSection: Section = {
  title: 'Diff',
  shortcuts: [
    ['] / [', 'Next / previous hunk'],
    ['|', 'Toggle the split view'],
    ['m', 'Toggle the preview of Markdown'],
  ],
}

// screens are the sections of the shortcuts of each route. The repository screen shows all of its views because they
// share the route.
const screens: { routeId: string; sections: Section[] }[] = [
  { routeId: '/agents', sections: [diffSection] },
  {
    routeId: '/repositories',
    sections: [
      {
        title: 'Log',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next revision'],
          ['k / ↑', 'Move the cursor to the previous revision'],
          ['@', 'Jump to the working copy'],
          ['Space', 'Check / uncheck the revision'],
          ['Esc', 'Uncheck all revisions'],
          ['/', 'Focus the revset'],
          ['r', 'Refresh'],
          ['n', 'New revision'],
          ['e', 'Edit the description'],
          ['c', 'Commit the working copy'],
          ['s', 'Split the revision'],
          ['J / K', 'Slide down / up the revision'],
          ['R', 'Rebase'],
          ['S', 'Squash'],
          ['B', 'Set the bookmark'],
          ['y', 'Copy the change ID'],
          ['v', 'Start the review of the revisions'],
          ['P', 'Create the pull request'],
          ['u', 'Undo'],
          ['f', 'Git fetch'],
          ['p', 'Git push'],
          ['F', 'Browse the files'],
          ['T', 'Toggle the files of the working copy'],
          ['2', 'Show the bookmarks'],
          ['4 / O', 'Show the operation log'],
          ['5 / E', 'Show the evolog'],
          ['w', 'Show the workspaces'],
          ['Cmd/Ctrl+K', 'Open the command palette'],
        ],
      },
      {
        title: 'Rebase / Squash',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next revision'],
          ['k / ↑', 'Move the cursor to the previous revision'],
          ['r / s / b', 'Rebase the revisions / the descendants / the branch'],
          ['o / a / i', 'Rebase onto / after / before the cursor'],
          ['c', 'Toggle combining the descriptions of the squash'],
          ['Enter', 'Rebase / squash'],
          ['Esc', 'Cancel'],
        ],
      },
      diffSection,
      {
        title: 'Bookmarks',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next bookmark'],
          ['k / ↑', 'Move the cursor to the previous bookmark'],
          ['d', 'Delete the bookmark'],
          ['f', 'Forget the bookmark'],
          ['t', 'Track / untrack the remote bookmark'],
          ['u', 'Undo'],
          ['1', 'Show the log'],
        ],
      },
      {
        title: 'Operation log / Evolog',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next entry'],
          ['k / ↑', 'Move the cursor to the previous entry'],
          ['1 / Esc', 'Show the log'],
        ],
      },
      {
        title: 'Workspaces',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next workspace'],
          ['k / ↑', 'Move the cursor to the previous workspace'],
          ['o', 'Open the workspace'],
          ['d', 'Remove the workspace'],
          ['f', 'Forget the workspace'],
          ['1 / Esc', 'Show the log'],
        ],
      },
      {
        title: 'Files',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next file'],
          ['k / ↑', 'Move the cursor to the previous file'],
          ['h / l', 'Fold / unfold the directory'],
          ['Enter', 'Open the file / toggle the directory'],
          ['H / L', 'Previous / next tab'],
          ['x', 'Close the tab'],
          ['m', 'Toggle the preview of Markdown'],
          ['/', 'Focus the filter'],
          ['1 / Esc', 'Show the log'],
        ],
      },
    ],
  },
  {
    routeId: '/reviews/$reviewId',
    sections: [
      {
        title: 'Review',
        shortcuts: [
          ['j / ↓', 'Move the cursor to the next line'],
          ['k / ↑', 'Move the cursor to the previous line'],
          ['c', 'Comment on the line of the cursor'],
          ['a', 'Toggle viewed on the file of the cursor'],
          ['Shift+F', 'Finish the review (approve if all comments are resolved)'],
          ['Click / Shift+Click', 'Select the line / the range to comment'],
          ['Esc', 'Clear the selection / the cursor'],
        ],
      },
      diffSection,
    ],
  },
]

// ShortcutHelpDialog shows the shortcuts of all screens and the current screen.
export const ShortcutHelpDialog = ({ open, onClose }: { open: boolean; onClose: () => void }) => {
  const routeId = useRouterState({ select: (s) => s.matches.at(-1)?.routeId })
  const sections = [globalSection, ...(screens.find((s) => s.routeId === routeId)?.sections ?? [])]

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Keyboard shortcuts</DialogTitle>
      <DialogContent>
        {sections.map((section) => (
          <Table key={section.title} size="small" sx={{ mb: 2 }}>
            <TableBody>
              <TableRow>
                <TableCell colSpan={2}>
                  <Typography variant="subtitle2">{section.title}</Typography>
                </TableCell>
              </TableRow>
              {section.shortcuts.map(([key, description]) => (
                <TableRow key={key}>
                  <TableCell sx={{ fontFamily: 'monospace', whiteSpace: 'nowrap' }}>{key}</TableCell>
                  <TableCell>{description}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ))}
      </DialogContent>
    </Dialog>
  )
}
