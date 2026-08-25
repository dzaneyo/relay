package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dzaneyo/relay/internal/model"
)

type TunnelHandle struct {
	LocalHost string
	LocalPort int
	close     func() error
}

func (h *TunnelHandle) Close() error {
	if h == nil || h.close == nil {
		return nil
	}
	return h.close()
}

type TunnelOpener interface {
	Open(context.Context, ResolvedSSHNode, string, int) (*TunnelHandle, error)
}

type OpenSSHTunnel struct{}

func NewOpenSSHTunnel() *OpenSSHTunnel { return &OpenSSHTunnel{} }

func (OpenSSHTunnel) Open(ctx context.Context, jump ResolvedSSHNode, targetHost string, targetPort int) (*TunnelHandle, error) {
	localPort, err := reserveLocalPort()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "relay-tunnel-*")
	if err != nil {
		return nil, fmt.Errorf("create SSH tunnel workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	plan := SSHPlan{Target: jump}
	configPath := filepath.Join(tempDir, "config")
	if err = os.WriteFile(configPath, []byte(buildOpenSSHConfig(plan)), 0o600); err != nil {
		cleanup()
		return nil, fmt.Errorf("write SSH tunnel config: %w", err)
	}

	forwardTarget := targetHost
	if strings.Contains(targetHost, ":") && !strings.HasPrefix(targetHost, "[") {
		forwardTarget = "[" + targetHost + "]"
	}
	sshArgs := []string{
		"-F", configPath,
		"-N",
		"-T",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-L", fmt.Sprintf("127.0.0.1:%d:%s:%d", localPort, forwardTarget, targetPort),
		"relay-target",
	}

	name := "ssh"
	args := sshArgs
	env := os.Environ()
	if jump.AuthType == model.AuthPassword && jump.Secret != "" {
		if _, lookErr := exec.LookPath("sshpass"); lookErr == nil {
			name = "sshpass"
			args = append([]string{"-e", "ssh"}, sshArgs...)
			env = replaceEnv(env, "SSHPASS="+jump.Secret)
		} else {
			askpassPath := filepath.Join(tempDir, "askpass.sh")
			if err = writeAskpassFiles(tempDir, askpassPath, plan); err != nil {
				cleanup()
				return nil, err
			}
			env = replaceEnv(env,
				"DISPLAY=relay",
				"SSH_ASKPASS="+askpassPath,
				"SSH_ASKPASS_REQUIRE=force",
			)
		}
	}

	path, err := exec.LookPath(name)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("%s client not found in PATH", name)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		cleanup()
		return nil, fmt.Errorf("start SSH tunnel: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	if err = waitForTunnel(ctx, localPort, waitCh); err != nil {
		_ = cmd.Process.Kill()
		cleanup()
		return nil, err
	}

	var once sync.Once
	handle := &TunnelHandle{LocalHost: "127.0.0.1", LocalPort: localPort}
	handle.close = func() error {
		var closeErr error
		once.Do(func() {
			if cmd.Process != nil {
				if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					closeErr = err
				}
			}
			select {
			case <-waitCh:
			case <-time.After(2 * time.Second):
			}
			cleanup()
		})
		return closeErr
	}
	return handle, nil
}

func reserveLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve local tunnel port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForTunnel(ctx context.Context, port int, waitCh <-chan error) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	address := net.JoinHostPort("127.0.0.1", fmt.Sprint(port))

	for {
		select {
		case err := <-waitCh:
			if err == nil {
				return errors.New("SSH tunnel exited before becoming ready")
			}
			return fmt.Errorf("SSH tunnel exited: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out waiting for SSH tunnel")
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}
