package cli

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/dzaneyo/relay/internal/app"
	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/service"
	webserver "github.com/dzaneyo/relay/internal/web"
	"github.com/spf13/cobra"
)

func printRecords(cmd *cobra.Command, items []model.Record) {
	for _, x := range items {
		if x.Category == model.CategoryNote {
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %s\n", x.Alias, x.Category, x.Name)
	}
}
func newListCommand(a *app.App) *cobra.Command {
	return &cobra.Command{Use: "list", Aliases: []string{"ls"}, Short: "List records", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		x, e := a.RecordService.List(cmd.Context(), "")
		if e == nil {
			printRecords(cmd, x)
		}
		return e
	}}
}
func newSearchCommand(a *app.App) *cobra.Command {
	return &cobra.Command{Use: "search <keyword>", Short: "Search records", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		x, e := a.RecordService.List(cmd.Context(), args[0])
		if e == nil {
			printRecords(cmd, x)
		}
		return e
	}}
}
func newShowCommand(a *app.App) *cobra.Command {
	return &cobra.Command{Use: "show <alias>", Short: "Show record details", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		d, e := a.RecordService.DetailByAlias(cmd.Context(), args[0])
		if e != nil {
			return e
		}
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "Name:       %s\nAlias:      %s\nCategory:   %s\n", d.Record.Name, d.Record.Alias, d.Record.Category)
		if d.SSH != nil {
			fmt.Fprintf(w, "Host:       %s\nPort:       %d\n", d.SSH.Host, d.SSH.Port)
		}
		if d.Database != nil {
			fmt.Fprintf(w, "DB Type:    %s\nHost:       %s\nPort:       %d\nDatabase:   %s\n", d.Database.DBType, d.Database.Host, d.Database.Port, d.Database.DatabaseName)
		}
		if d.Credential != nil {
			fmt.Fprintf(w, "Username:   %s\nAuth:       %s\n", d.Credential.Username, d.Credential.AuthType)
			if d.Credential.AuthType == model.AuthPassword {
				fmt.Fprintln(w, "Password:   ******")
			}
			if d.Credential.KeyPath != "" {
				fmt.Fprintf(w, "Key Path:   %s\n", d.Credential.KeyPath)
			}
		}
		if d.SSH != nil && d.SSH.RouteID != "" {
			routeName := d.SSH.RouteID
			if route, err := a.Repo.FindRoute(cmd.Context(), d.SSH.RouteID); err == nil {
				routeName = route.Name
			}
			fmt.Fprintf(w, "Route:      %s\n", routeName)
		}
		return nil
	}}
}
func printNode(w interface{ Write([]byte) (int, error) }, prefix string, n service.ResolvedSSHNode) {
	fmt.Fprintf(w, "%s%s  %s@%s:%d  %s", prefix, n.Alias, n.Username, n.Host, n.Port, n.AuthType)
	if n.KeyPath != "" {
		fmt.Fprintf(w, " %s", n.KeyPath)
	}
	fmt.Fprintln(w)
}
func newConnectCommand(a *app.App) *cobra.Command {
	return &cobra.Command{Use: "connect <alias>", Short: "Connect to a HOST or DATABASE", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		plan, e := a.Connect.Connect(cmd.Context(), args[0])
		if plan != nil {
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Route:")
			for i, n := range plan.Hops {
				printNode(w, fmt.Sprintf("  %d. ", i+1), n)
			}
			fmt.Fprintln(w, "Target:")
			printNode(w, "  ", plan.Target)
		}
		if errors.Is(e, service.ErrSSHLaunchNotEnabled) {
			fmt.Fprintln(cmd.OutOrStdout(), e)
			return nil
		}
		return e
	}}
}
func newWebCommand(a *app.App) *cobra.Command {
	return &cobra.Command{Use: "web", Short: "Start local web UI", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		addr := "127.0.0.1:17321"
		fmt.Fprintln(cmd.OutOrStdout(), "relay Web: http://"+addr)
		return http.ListenAndServe(addr, webserver.NewServer(a).Handler())
	}}
}
