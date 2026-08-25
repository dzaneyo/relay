package service

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/dzaneyo/relay/internal/model"
)

type sshRunner struct {
	spec   CommandSpec
	config string
	answer string
}

func (r *sshRunner) Run(_ context.Context, spec CommandSpec) error {
	r.spec = spec
	data, err := os.ReadFile(spec.Args[1])
	if err != nil {
		return err
	}
	r.config = string(data)
	for _, value := range spec.Env {
		if strings.HasPrefix(value, "SSH_ASKPASS=") {
			askpass := strings.TrimPrefix(value, "SSH_ASKPASS=")
			cmd := exec.Command(askpass, "edm@target.local's password:")
			cmd.Env = spec.Env
			answer, err := cmd.Output()
			if err != nil {
				return err
			}
			r.answer = string(answer)
			break
		}
	}
	return nil
}

func TestOpenSSHLauncherBuildsPasswordAndJumpRouteConfig(t *testing.T) {
	runner := new(sshRunner)
	launcher := NewOpenSSHLauncher(runner)
	plan := SSHPlan{
		Hops: []ResolvedSSHNode{
			{Host: "jump-1.local", Port: 22, Username: "jump1", AuthType: model.AuthPassword, Secret: "jump-secret"},
			{Host: "jump-2.local", Port: 2202, Username: "jump2", AuthType: model.AuthSSHKey, KeyPath: "~/.ssh/jump2"},
		},
		Target: ResolvedSSHNode{Host: "target.local", Port: 22, Username: "edm", AuthType: model.AuthPassword, Secret: "target-secret"},
	}

	if err := launcher.Launch(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if runner.spec.Name != "ssh" || len(runner.spec.Args) != 3 || runner.spec.Args[2] != "relay-target" {
		t.Fatalf("unexpected SSH command: %+v", runner.spec)
	}
	for _, want := range []string{
		"Host relay-hop-1",
		"PreferredAuthentications password,keyboard-interactive",
		"IdentityFile \"~/.ssh/jump2\"",
		"ProxyJump relay-hop-1,relay-hop-2",
	} {
		if !strings.Contains(runner.config, want) {
			t.Fatalf("SSH config missing %q:\n%s", want, runner.config)
		}
	}
	if strings.Contains(runner.config, "jump-secret") || strings.Contains(runner.config, "target-secret") {
		t.Fatal("password leaked into SSH config")
	}
	if runner.answer != "target-secret" {
		t.Fatal("saved target password was not supplied through SSH_ASKPASS")
	}
	if strings.Contains(strings.Join(runner.spec.Args, " "), "target-secret") || strings.Contains(strings.Join(runner.spec.Env, " "), "target-secret") {
		t.Fatal("password leaked into SSH argv or environment")
	}
}

func TestDirectPasswordUsesSSHPassSpec(t *testing.T) {
	target := ResolvedSSHNode{
		Host: "172.17.132.33", Port: 22, Username: "edm",
		AuthType: model.AuthPassword, Secret: "saved-password",
	}
	spec := buildSSHPassSpec(target, []string{"PATH=/opt/homebrew/bin", "SSHPASS=old"})
	if spec.Name != "sshpass" || !strings.Contains(strings.Join(spec.Args, " "), "edm@172.17.132.33") {
		t.Fatalf("unexpected sshpass command: %+v", spec)
	}
	if strings.Contains(strings.Join(spec.Args, " "), "saved-password") {
		t.Fatal("password leaked into SSH argv")
	}
	if got := strings.Join(spec.Env, " "); !strings.Contains(got, "SSHPASS=saved-password") || strings.Contains(got, "SSHPASS=old") {
		t.Fatalf("SSHPASS environment was not replaced: %v", spec.Env)
	}
}

func TestSingleJumpUsesSSHPassOnlyForOnePassword(t *testing.T) {
	plan := SSHPlan{
		Hops:   []ResolvedSSHNode{{AuthType: model.AuthSSHKey}},
		Target: ResolvedSSHNode{AuthType: model.AuthPassword, Secret: "target-password"},
	}
	if password, ok := singleJumpSSHPassPassword(plan); !ok || password != "target-password" {
		t.Fatal("single-jump password should use sshpass")
	}
	plan.Hops[0] = ResolvedSSHNode{AuthType: model.AuthPassword, Secret: "jump-password"}
	if _, ok := singleJumpSSHPassPassword(plan); ok {
		t.Fatal("different jump and target passwords must use SSH_ASKPASS")
	}
}
