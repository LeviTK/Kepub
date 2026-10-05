package validation

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Called only after group SIGKILL: Z/X cannot write, and an orphan zombie may
// persist in containers. Other states or an unreadable process table fail closed.
func backendGroupAlive(pgid int) (bool, error) {
	e := syscall.Kill(-pgid, 0)
	if errors.Is(e, syscall.ESRCH) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	entries, e := os.ReadDir("/proc")
	if e != nil {
		return false, e
	}
	for _, entry := range entries {
		if _, e := strconv.Atoi(entry.Name()); e != nil {
			continue
		}
		b, e := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if errors.Is(e, os.ErrNotExist) || errors.Is(e, syscall.ESRCH) {
			continue
		}
		if e != nil {
			return false, e
		}
		end := strings.LastIndexByte(string(b), ')')
		if end < 0 {
			return false, fmt.Errorf("invalid process stat")
		}
		fields := strings.Fields(string(b[end+1:]))
		if len(fields) < 3 {
			return false, fmt.Errorf("short process stat")
		}
		group, e := strconv.Atoi(fields[2])
		if e != nil {
			return false, e
		}
		if group == pgid && fields[0] != "Z" && fields[0] != "X" {
			return true, nil
		}
	}
	return false, nil
}
