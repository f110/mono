package monodev

import (
	"github.com/spf13/cobra"
)

type Manager []*cobra.Command

var CommandManager = make(Manager, 0)

func (m Manager) Add(cmd *cobra.Command) {
	for _, v := range m {
		cmd.AddCommand(v)
	}
}

func (m *Manager) Register(cmd *cobra.Command) {
	*m = append(*m, cmd)
}
