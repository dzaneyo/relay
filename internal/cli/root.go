package cli

import (
	"github.com/dzaneyo/relay/internal/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(a *app.App) *cobra.Command {
	root := &cobra.Command{
		Use:   "relay [alias]",
		Short: "Local account and connection manager",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(a, cmd, args)
		},
	}
	root.AddCommand(newListCommand(a), newSearchCommand(a), newShowCommand(a), newConnectCommand(a), newCheckCommand(a), newWebCommand(a))
	return root
}
