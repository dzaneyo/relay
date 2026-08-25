package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dzaneyo/relay/internal/model"
)

type OpenSSHLauncher struct {
	runner CommandRunner
}

func NewOpenSSHLauncher(runner CommandRunner) *OpenSSHLauncher {
	return &OpenSSHLauncher{runner: runner}
}

func (l *OpenSSHLauncher) Launch(ctx context.Context, plan SSHPlan) error {
	if canUseSSHPass(plan) {
		if _, err := exec.LookPath("sshpass"); err == nil {
			return l.runner.Run(ctx, buildSSHPassSpec(plan.Target, os.Environ()))
		}
	}

	tempDir, err := os.MkdirTemp("", "relay-ssh-*")
	if err != nil {
		return fmt.Errorf("create SSH workspace: %w", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config")
	if err = os.WriteFile(configPath, []byte(buildOpenSSHConfig(plan)), 0600); err != nil {
		return fmt.Errorf("write SSH config: %w", err)
	}

	env := os.Environ()
	if password, ok := singleJumpSSHPassPassword(plan); ok {
		if _, lookPathErr := exec.LookPath("sshpass"); lookPathErr == nil {
			spec := CommandSpec{
				Name: "sshpass",
				Args: []string{"-e", "ssh", "-F", configPath, "relay-target"},
				Env:  replaceEnv(env, "SSHPASS="+password),
			}
			return l.runner.Run(ctx, spec)
		}
	}
	if passwordNodes(plan) > 0 {
		askpassPath := filepath.Join(tempDir, "askpass.sh")
		if err = writeAskpassFiles(tempDir, askpassPath, plan); err != nil {
			return err
		}
		env = replaceEnv(env,
			"DISPLAY=relay",
			"SSH_ASKPASS="+askpassPath,
			"SSH_ASKPASS_REQUIRE=force",
		)
	}

	spec := CommandSpec{
		Name: "ssh",
		Args: []string{"-F", configPath, "relay-target"},
		Env:  env,
	}
	return l.runner.Run(ctx, spec)
}

func canUseSSHPass(plan SSHPlan) bool {
	return len(plan.Hops) == 0 && plan.Target.AuthType == model.AuthPassword && plan.Target.Secret != ""
}

func singleJumpSSHPassPassword(plan SSHPlan) (string, bool) {
	if len(plan.Hops) != 1 {
		return "", false
	}
	password := ""
	for _, node := range []ResolvedSSHNode{plan.Hops[0], plan.Target} {
		if node.AuthType != model.AuthPassword || node.Secret == "" {
			continue
		}
		if password != "" && password != node.Secret {
			return "", false
		}
		password = node.Secret
	}
	return password, password != ""
}

func buildSSHPassSpec(target ResolvedSSHNode, env []string) CommandSpec {
	args := []string{
		"-e",
		"ssh",
		"-p", fmt.Sprint(target.Port),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "PreferredAuthentications=password,keyboard-interactive",
		"-o", "PubkeyAuthentication=no",
	}
	destination := target.Host
	if target.Username != "" {
		destination = target.Username + "@" + target.Host
	}
	args = append(args, destination)
	return CommandSpec{
		Name: "sshpass",
		Args: args,
		Env:  replaceEnv(env, "SSHPASS="+target.Secret),
	}
}

func passwordNodes(plan SSHPlan) int {
	count := 0
	for _, node := range append(append([]ResolvedSSHNode{}, plan.Hops...), plan.Target) {
		if node.AuthType == model.AuthPassword && node.Secret != "" {
			count++
		}
	}
	return count
}

func writeAskpassFiles(tempDir, askpassPath string, plan SSHPlan) error {
	nodes := append(append([]ResolvedSSHNode{}, plan.Hops...), plan.Target)
	var script strings.Builder
	script.WriteString("#!/bin/sh\nprompt=$1\ncase \"$prompt\" in\n")
	passwordIndex := 0
	for _, node := range nodes {
		if node.AuthType != model.AuthPassword || node.Secret == "" {
			continue
		}
		passwordPath := filepath.Join(tempDir, fmt.Sprintf("password-%d", passwordIndex))
		if err := os.WriteFile(passwordPath, []byte(node.Secret), 0600); err != nil {
			return fmt.Errorf("write SSH password: %w", err)
		}
		identity := node.Host
		if node.Username != "" {
			identity = node.Username + "@" + node.Host
		}
		fmt.Fprintf(&script, "  *%s*) cat %s; exit 0 ;;\n", shellQuote(identity), shellQuote(passwordPath))
		passwordIndex++
	}
	script.WriteString("esac\n")
	script.WriteString("if [ -r /dev/tty ]; then\n")
	script.WriteString("  printf '%s ' \"$prompt\" > /dev/tty\n")
	script.WriteString("  stty -echo < /dev/tty\n")
	script.WriteString("  IFS= read -r answer < /dev/tty\n")
	script.WriteString("  stty echo < /dev/tty\n")
	script.WriteString("  printf '\\n' > /dev/tty\n")
	script.WriteString("  printf '%s\\n' \"$answer\"\n")
	script.WriteString("  exit 0\n")
	script.WriteString("fi\nexit 1\n")
	if err := os.WriteFile(askpassPath, []byte(script.String()), 0700); err != nil {
		return fmt.Errorf("write SSH askpass helper: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func replaceEnv(env []string, values ...string) []string {
	keys := make(map[string]struct{}, len(values))
	for _, value := range values {
		if i := strings.IndexByte(value, '='); i >= 0 {
			keys[value[:i]] = struct{}{}
		}
	}
	out := make([]string, 0, len(env)+len(values))
	for _, value := range env {
		key := value
		if i := strings.IndexByte(value, '='); i >= 0 {
			key = value[:i]
		}
		if _, replaced := keys[key]; !replaced {
			out = append(out, value)
		}
	}
	return append(out, values...)
}

func buildOpenSSHConfig(plan SSHPlan) string {
	var b strings.Builder
	for i, hop := range plan.Hops {
		writeSSHHost(&b, fmt.Sprintf("relay-hop-%d", i+1), hop)
	}
	writeSSHHost(&b, "relay-target", plan.Target)
	if len(plan.Hops) > 0 {
		aliases := make([]string, len(plan.Hops))
		for i := range plan.Hops {
			aliases[i] = fmt.Sprintf("relay-hop-%d", i+1)
		}
		fmt.Fprintf(&b, "    ProxyJump %s\n", strings.Join(aliases, ","))
	}
	return b.String()
}

func writeSSHHost(b *strings.Builder, alias string, node ResolvedSSHNode) {
	fmt.Fprintf(b, "Host %s\n", alias)
	fmt.Fprintf(b, "    HostName %s\n", sshConfigValue(node.Host))
	fmt.Fprintf(b, "    Port %d\n", node.Port)
	if node.Username != "" {
		fmt.Fprintf(b, "    User %s\n", sshConfigValue(node.Username))
	}
	switch node.AuthType {
	case model.AuthPassword:
		b.WriteString("    PreferredAuthentications password,keyboard-interactive\n")
		b.WriteString("    PubkeyAuthentication no\n")
	case model.AuthSSHKey:
		fmt.Fprintf(b, "    IdentityFile %s\n", sshConfigValue(node.KeyPath))
		b.WriteString("    IdentitiesOnly yes\n")
	}
}

func sshConfigValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return "\"" + value + "\""
}
