package util

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"
)

// hasControllingTerminal reports whether sudo can prompt on /dev/tty, meaning
// um is in the terminal's foreground process group. Redirected stdin still
// counts. A terminal inherited by a GUI app (e.g. System Settings launched from
// Konsole) doesn't, since the prompt would land in the wrong place or SIGTTIN um.
func hasControllingTerminal() bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer tty.Close()
	pgrp, err := unix.IoctlGetInt(int(tty.Fd()), unix.TIOCGPGRP)
	return err == nil && pgrp == unix.Getpgrp()
}

// CanPrompt reports whether um can ask the user for confirmation or a password
// in the terminal
func CanPrompt() bool {
	return hasControllingTerminal()
}

// inAdminGroup reports whether the user is in wheel (or sudo on Debian-likes)
func inAdminGroup() bool {
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

// sudoWithoutPassword reports whether sudo works without prompting (NOPASSWD
// or cached credentials). It probes with `true`, so a NOPASSWD rule that covers
// `true` but not um passes here and the real `sudo -n` fails. That setup is rare
// enough to accept.
//
// the probe only runs for admin group members. For anyone else sudo would log
// a "user NOT in sudoers" incident on every call, and pkexec is the right
// choice anyway.
func sudoWithoutPassword() bool {
	return inAdminGroup() && exec.Command("/usr/bin/sudo", "-n", "true").Run() == nil
}

// SudoIfNeeded re-executes um as root if it isn't already. It only returns if
// um is already root (nil) or escalating failed.
//
// sudo is used when it can prompt or doesn't need a password, and forwardEnv is
// passed through. Otherwise (e.g. from the Plasma KCMs) it uses pkexec for a
// graphical polkit prompt. pkexec gets a clean environment, since wrapping um
// in `env` would stop the polkit action from matching, so um refuses to use it
// while any forwardEnv var is set rather than silently running as root against
// different paths. pkexec exits 126 if the prompt is dismissed and 127 if
// authorization fails.
func SudoIfNeeded(forwardEnv []string) error {
	if os.Getuid() == 0 {
		return nil
	}

	current, err := os.Executable()
	if err != nil {
		return err
	}

	sudoArgs := []string{"sudo"}
	if !hasControllingTerminal() {
		if sudoWithoutPassword() {
			sudoArgs = append(sudoArgs, "-n")
		} else if pkexec, err := exec.LookPath("pkexec"); err == nil {
			for _, v := range forwardEnv {
				if _, found := os.LookupEnv(v); found {
					return fmt.Errorf("%s is set, but pkexec would drop it. Run um from a terminal or unset it", v)
				}
			}
			args := append([]string{"pkexec", current}, os.Args[1:]...)
			return unix.Exec(pkexec, args, os.Environ())
		}
		// no pkexec either, so fall through to sudo, which errors out instead of hanging
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
