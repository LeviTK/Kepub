//go:build linux || darwin

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

// Presence includes zombies: before cleanup, uncertain residual members must
// conservatively block completion rather than race a live process snapshot.
func groupPresent(pid int) (bool, error) {
	err := syscall.Kill(-pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	return err == nil, err
}

func signalGroup(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// stopGroup does not equate cmd.Wait with absence of writers. Even a normal
// exit is followed by group cleanup. Absence is checked independently of pipes.
func stopGroup(pid int, grace, timeout time.Duration) bool {
	if err := signalGroup(pid, syscall.SIGTERM); err != nil {
		return false
	}
	killAt, end := time.Now().Add(grace), time.Now().Add(grace+timeout)
	killed := false
	for {
		present, err := groupPresent(pid)
		if err != nil {
			return false
		}
		if !present {
			return true
		}
		if killed {
			// Only inspect Linux zombie-only groups after SIGKILL, when live
			// members can no longer fork new group members during a snapshot.
			alive, err := groupAlive(pid)
			if err != nil {
				return false
			}
			if !alive {
				return true
			}
		}
		if !killed && !time.Now().Before(killAt) {
			if err := signalGroup(pid, syscall.SIGKILL); err != nil {
				return false
			}
			killed = true
		}
		if !time.Now().Before(end) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
