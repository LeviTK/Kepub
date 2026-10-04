//go:build !linux && !darwin

package main

import (
	"errors"
	"os/exec"
	"time"
)

func configureProcess(*exec.Cmd) error                 { return errors.New("process groups require Linux or macOS") }
func stopGroup(int, time.Duration, time.Duration) bool { return false }
func groupPresent(int) (bool, error) {
	return false, errors.New("process groups require Linux or macOS")
}
