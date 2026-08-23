package cli

import (
	"github.com/dzaneyo/relay/internal/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(a *app.App) *cobra.Command {
	root := &cobra.Command{Use: "relay", Short: "Local account and connection manager", Args: cobra.NoArgs}
	root.AddCommand(newListCommand(a), newSearchCommand(a), newShowCommand(a), newConnectCommand(a), newWebCommand(a))
	return root
}
