//go:build linux

package main

import "syscall"

func enableReaping() error {
	// PR_SET_CHILD_SUBREAPER: orphaned CLI/tool processes are adopted by this
	// standalone supervisor rather than leaving zombies for the orb's PID 1.
	_, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, 36, 1, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func reapGroup(pgid int) {
	// Called only after cmd.Wait has reaped the Node child, avoiding wait races.
	for {
		pid, err := syscall.Wait4(-pgid, nil, syscall.WNOHANG, nil)
		if err != nil || pid <= 0 {
			return
		}
	}
}
