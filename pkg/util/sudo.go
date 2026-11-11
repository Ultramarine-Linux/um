package util

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"
)

// a GUI app can inherit a terminal it isn't in the foreground of
func CanPrompt() bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer tty.Close()
	pgrp, err := unix.IoctlGetInt(int(tty.Fd()), unix.TIOCGPGRP)
	return err == nil && pgrp == unix.Getpgrp()
}

func inWheelOrSudoGroup() bool {
	gids, err := os.Getgroups()
	if err != nil {
		return false
	}
	gids = append(gids, os.Getegid())
	for _, name := range []string{"wheel", "sudo"} {
		g, err := user.LookupGroup(name)
		if err != nil {
			continue
		}
		for _, gid := range gids {
			if strconv.Itoa(gid) == g.Gid {
				return true
			}
		}
	}
	return false
}

// non-sudoers are skipped since sudo logs an incident for each attempt
func sudoWithoutPassword() bool {
	return inWheelOrSudoGroup() && exec.Command("/usr/bin/sudo", "-n", "true").Run() == nil
}

// pkexec drops the environment, and wrapping um in `env` would break the polkit action match
func execPkexec(pkexec, um string, forwardEnv []string) error {
	for _, v := range forwardEnv {
		if _, found := os.LookupEnv(v); found {
			return fmt.Errorf("%s is set, but pkexec would drop it. Run um from a terminal or unset it", v)
		}
	}
	args := append([]string{"pkexec", um}, os.Args[1:]...)
	return unix.Exec(pkexec, args, os.Environ())
}

func SudoIfNeeded(forwardEnv []string) error {
	if os.Getuid() == 0 {
		return nil
	}

	current, err := os.Executable()
	if err != nil {
		return err
	}

	sudoArgs := []string{"sudo"}
	if !CanPrompt() {
		if sudoWithoutPassword() {
			sudoArgs = append(sudoArgs, "-n")
		} else if pkexec, err := exec.LookPath("pkexec"); err == nil {
			return execPkexec(pkexec, current, forwardEnv)
		}
	}

	for _, v := range forwardEnv {
		val, found := os.LookupEnv(v)
		if !found {
			continue
		}

		sudoArgs = append(sudoArgs, v+"="+val)
	}

	args := append(sudoArgs, current)
	args = append(args, os.Args[1:]...)

	return unix.Exec("/usr/bin/sudo", args, os.Environ())
}
