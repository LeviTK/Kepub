package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func groupAlive(pgid int) (bool, error) {
	err := syscall.Kill(-pgid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Orphan zombies can outlive the group leader in containers whose PID 1
	// does not reap promptly. Zombies cannot write. Do not confuse them with
	// live descendants, and fail closed on an unreadable process table.
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		// comm may contain spaces and parentheses; fields after its final ')'
		// begin with state, ppid, pgrp.
		i := strings.LastIndexByte(string(stat), ')')
		if i < 0 {
			return false, fmt.Errorf("invalid /proc stat")
		}
		fields := strings.Fields(string(stat[i+1:]))
		if len(fields) < 3 {
			return false, fmt.Errorf("short /proc stat")
		}
		group, err := strconv.Atoi(fields[2])
		if err != nil {
			return false, err
		}
		if group == pgid && fields[0] != "Z" && fields[0] != "X" {
			return true, nil
		}
	}
	return false, nil
}
