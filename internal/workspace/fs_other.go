//go:build !linux && !darwin

package workspace

import (
	"errors"
	"os"
)

var errPlatform = errors.New("workspace supports Linux and macOS only")

func openRegular(*os.Root, string) (*os.File, error) { return nil, errPlatform }
func lock(*os.Root) (*os.File, error)                { return nil, errPlatform }
func publish(*os.Root, string, string) error         { return errPlatform }
