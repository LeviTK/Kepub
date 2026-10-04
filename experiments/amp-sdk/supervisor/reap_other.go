//go:build !linux

package main

import "errors"

func enableReaping() error { return errors.New("process reclamation verified on Linux only") }
func reapGroup(int)        {}
