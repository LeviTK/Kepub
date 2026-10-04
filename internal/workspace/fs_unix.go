//go:build linux || darwin

package workspace

import (
	"errors"
	"fmt"
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

func openRegular(r *os.Root, name string) (*os.File, error) {
	before, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %q", name)
	}
	f, err := r.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err == nil && (!after.Mode().IsRegular() || !os.SameFile(before, after) || after.Sys().(*syscall.Stat_t).Nlink != 1) {
		err = fmt.Errorf("changed or hard-linked file: %q", name)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func lock(r *os.Root) (*os.File, error) {
	// Never unlink this inode, including on Close: otherwise a waiting opener
	// and a new opener can lock different files for the same workspace.
	f, err := r.OpenFile("owner.lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	i, err := f.Stat()
	if err == nil && (!i.Mode().IsRegular() || i.Sys().(*syscall.Stat_t).Nlink != 1) {
		err = fmt.Errorf("unsafe workspace lock")
	}
	if err == nil {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			err = ErrBusy
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Both parents are opened and confined before the no-replace system call. Only
// single components go to renameat; the operation cannot follow a leaf symlink.
func publish(r *os.Root, from, to string) error {
	a, err := subdir(r, path.Dir(from))
	if err != nil {
		return err
	}
	defer a.Close()
	b, err := subdir(r, path.Dir(to))
	if err != nil {
		return err
	}
	defer b.Close()
	af, err := a.Open(".")
	if err != nil {
		return err
	}
	defer af.Close()
	bf, err := b.Open(".")
	if err != nil {
		return err
	}
	defer bf.Close()
	return renameNoReplace(int(af.Fd()), path.Base(from), int(bf.Fd()), path.Base(to))
}
