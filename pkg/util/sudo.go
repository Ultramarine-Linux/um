package util

import (
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// hasControllingTerminal reports whether sudo can prompt for a password: the
// process has a controlling terminal and is in its foreground process group.
// sudo prompts through /dev/tty rather than stdin, so redirected stdin
// (`um ... </dev/null`) still counts. A terminal merely inherited by a GUI app
// (e.g. System Settings launched from Konsole, possibly backgrounded) doesn't:
// the prompt would land in the wrong place or stop um with SIGTTIN.
func hasControllingTerminal() bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer tty.Close()
	pgrp, err := unix.IoctlGetInt(int(tty.Fd()), unix.TIOCGPGRP)
	return err == nil && pgrp == unix.Getpgrp()
}

// sudoWithoutPassword reports whether sudo can run without prompting, e.g.
// because of a NOPASSWD rule or cached credentials. This probes with `true`,
// so a NOPASSWD rule that covers `true` but not um passes here and the real
// `sudo -n` then fails; that's an unusual enough setup to accept.
func sudoWithoutPassword() bool {
	return exec.Command("/usr/bin/sudo", "-n", "true").Run() == nil
}

// SudoIfNeeded re-executes um as root if it isn't already.
//
// If sudo can be used (there's a terminal to prompt on, or sudo doesn't need
// a password), it uses sudo and forwards forwardEnv. Otherwise (e.g. when
// started by the Plasma KCMs) it uses pkexec, which shows a graphical polkit
// prompt. pkexec runs um with a clean environment and forwardEnv is not
// forwarded: wrapping um in `env` would stop the polkit action for /usr/bin/um
// from matching, and not trusting the caller's environment is the point.
// pkexec exits with 126 if the prompt is dismissed and 127 if authorization
// fails.
func SudoIfNeeded(forwardEnv []string) {
	if os.Getuid() == 0 {
		return
	}

	current, err := os.Executable()
	if err != nil {
		panic(err)
	}

	sudoArgs := []string{"sudo"}
	if !hasControllingTerminal() {
		if sudoWithoutPassword() {
			sudoArgs = append(sudoArgs, "-n")
		} else if pkexec, err := exec.LookPath("pkexec"); err == nil {
			args := append([]string{"pkexec", current}, os.Args[1:]...)
			if err := unix.Exec(pkexec, args, os.Environ()); err != nil {
				panic(err)
			}
		}
		// No pkexec either: fall through to sudo, which fails with a
		// "terminal is required" error instead of hanging.
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

	if err := unix.Exec("/usr/bin/sudo", args, os.Environ()); err != nil {
		panic(err)
	}
}
