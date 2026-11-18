import { useTheme } from '@mui/material'

export const cellWidth = 8
const nodeRunes = ['@', '○', '◆', '×', '◌']
// downward and upward are the characters which extend to the bottom and the top of the cell. The node connects to the
// neighboring lines only through them.
const downward = new Set(['│', '╮', '╭', '├', '┤', '┬', '┼', ...nodeRunes])
const upward = new Set(['│', '╯', '╰', '├', '┤', '┴', '┼', '~', ...nodeRunes])

// GraphGutter draws a line of the graph of jj log. Each character of the gutter is a cell. The characters of the box
// drawing are converted into the segments between the center and the edges of the cell. above and below are the
// gutters of the neighboring lines to connect the node to them.
export const GraphGutter = ({
  gutter,
  above,
  below,
  height,
  width,
  workingCopy,
}: {
  gutter: string
  above: string | undefined
  below: string | undefined
  height: number
  width: number
  workingCopy: boolean
}) => {
  // The colors of theme.vars are CSS variables which follow the color scheme. Only the style can use them.
  const theme = useTheme()
  const palette = theme.vars?.palette ?? theme.palette
  const color = palette.text.secondary
  const chars = [...gutter]
  const cx = (i: number) => i * cellWidth + cellWidth / 2
  const cy = height / 2
  const charAt = (line: string | undefined, i: number) => (line ? [...line][i] : undefined) ?? ''

  const elements = chars.map((c, i) => {
    const x = cx(i)
    const left = i * cellWidth
    const right = left + cellWidth
    const key = `${i}`
    switch (c) {
      case '│':
        return <line key={key} x1={x} y1={0} x2={x} y2={height} />
      case '─':
        return <line key={key} x1={left} y1={cy} x2={right} y2={cy} />
      case '╮':
        return <path key={key} d={`M ${left} ${cy} Q ${x} ${cy} ${x} ${height}`} fill="none" />
      case '╯':
        return <path key={key} d={`M ${left} ${cy} Q ${x} ${cy} ${x} 0`} fill="none" />
      case '╭':
        return <path key={key} d={`M ${right} ${cy} Q ${x} ${cy} ${x} ${height}`} fill="none" />
      case '╰':
        return <path key={key} d={`M ${right} ${cy} Q ${x} ${cy} ${x} 0`} fill="none" />
      case '├':
        return <path key={key} d={`M ${x} 0 V ${height} M ${x} ${cy} H ${right}`} fill="none" />
      case '┤':
        return <path key={key} d={`M ${x} 0 V ${height} M ${x} ${cy} H ${left}`} fill="none" />
      case '┬':
        return <path key={key} d={`M ${left} ${cy} H ${right} M ${x} ${cy} V ${height}`} fill="none" />
      case '┴':
        return <path key={key} d={`M ${left} ${cy} H ${right} M ${x} ${cy} V 0`} fill="none" />
      case '┼':
        return <path key={key} d={`M ${left} ${cy} H ${right} M ${x} 0 V ${height}`} fill="none" />
      case '~':
        return <line key={key} x1={x} y1={0} x2={x} y2={cy} strokeDasharray="2 2" />
    }
    if (!nodeRunes.includes(c)) {
      return null
    }
    const r = 4
    return (
      <g key={key}>
        {downward.has(charAt(above, i)) && <line x1={x} y1={0} x2={x} y2={cy - r} />}
        {upward.has(charAt(below, i)) && <line x1={x} y1={cy + r} x2={x} y2={height} />}
        {c === '◆' ? (
          <rect x={x - r} y={cy - r} width={r * 2} height={r * 2} transform={`rotate(45 ${x} ${cy})`} style={{ fill: color }} />
        ) : (
          <circle
            cx={x}
            cy={cy}
            r={r}
            style={{ fill: workingCopy ? palette.primary.main : c === '×' ? palette.error.main : 'none' }}
            strokeDasharray={c === '◌' ? '2 2' : undefined}
          />
        )}
      </g>
    )
  })

  return (
    <svg width={width} height={height} strokeWidth={1.5} style={{ flexShrink: 0, display: 'block', stroke: color }}>
      {elements}
    </svg>
  )
}
