package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dzaneyo/relay/internal/app"
	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/service"
	"github.com/spf13/cobra"
)

func runRoot(a *app.App, cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		return connectAlias(a, cmd, args[0])
	}
	return runPicker(a, cmd)
}

func runPicker(a *app.App, cmd *cobra.Command) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	query := ""

	for {
		items, err := a.Repo.ListConnectable(cmd.Context(), strings.TrimSpace(query), 20)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			if query == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No HOST or DATABASE records yet.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "No matches for %q.\n", query)
		}

		if len(items) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Connect:")
			for i, item := range items {
				favorite := " "
				if item.Favorite {
					favorite = "★"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %s %-20s %-10s %s\n", i+1, favorite, item.Alias, item.Category, item.Name)
			}
		}
		fmt.Fprint(cmd.OutOrStdout(), "Select number, type to search, or q to quit: ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return nil
		}
		line = strings.TrimSpace(line)
		if line == "q" || line == "quit" || line == "exit" {
			return nil
		}
		if n, convErr := strconv.Atoi(line); convErr == nil {
			if n < 1 || n > len(items) {
				fmt.Fprintln(cmd.OutOrStdout(), "Invalid selection.")
				continue
			}
			return connectAlias(a, cmd, items[n-1].Alias)
		}
		query = line
	}
}

func connectAlias(a *app.App, cmd *cobra.Command, alias string) error {
	plan, err := a.Connect.Connect(cmd.Context(), strings.TrimSpace(alias))
	if plan != nil {
		printPlan(cmd, plan)
	}
	return err
}

func printPlan(cmd *cobra.Command, plan *service.SSHPlan) {
	w := cmd.OutOrStdout()
	if len(plan.Hops) > 0 {
		fmt.Fprintln(w, "Route:")
		for i, n := range plan.Hops {
			printNode(w, fmt.Sprintf("  %d. ", i+1), n)
		}
	}
	fmt.Fprintln(w, "Target:")
	printNode(w, "  ", plan.Target)
}

func printCheckReport(cmd *cobra.Command, report *service.CheckReport) error {
	fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", report.Alias, report.Category)
	for _, item := range report.Items {
		mark := "✓"
		switch item.Status {
		case service.CheckWarn:
			mark = "!"
		case service.CheckFail:
			mark = "✗"
		}
		if item.Message == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", mark, item.Name)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s %-24s %s\n", mark, item.Name, item.Message)
		}
	}
	if !report.Ready() {
		return errors.New("connection check failed")
	}
	return nil
}

var _ = model.CategoryHost
