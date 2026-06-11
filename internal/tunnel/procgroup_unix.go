//go:build unix

package tunnel

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup signals the child's entire process group (negative PID) with
// SIGTERM, then escalates to SIGKILL if it has not exited within grace. After
// it returns, no child of the group remains.
func killGroup(cmd *exec.Cmd, grace time.Duration) error {
	pgid := cmd.Process.Pid

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
	}

	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	<-done
	return nil
}
