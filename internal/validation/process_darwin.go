package validation

import (
	"errors"
	"syscall"
)

func backendGroupAlive(pgid int) (bool, error) {
	e := syscall.Kill(-pgid, 0)
	if errors.Is(e, syscall.ESRCH) {
		return false, nil
	}
	return e == nil, e
}
