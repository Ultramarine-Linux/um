package util

import (
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// isInteractive reports whether stdin is a terminal, i.e. whether sudo can prompt.
func isInteractive() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return err == nil
}

// SudoIfNeeded re-executes um as root if it isn't already.
//
// From a terminal it uses sudo and forwards forwardEnv. Without a terminal (e.g.
// when started by the Plasma KCMs) sudo can't prompt, so it uses pkexec, which
// shows a graphical polkit prompt. pkexec runs um with a clean environment and
// forwardEnv is not forwarded: wrapping um in `env` would stop the polkit action
// for /usr/bin/um from matching. pkexec exits with 126 if the prompt is dismissed
// and 127 if authorization fails.
func SudoIfNeeded(forwardEnv []string) {
	if os.Getuid() == 0 {
		return
	}

	current, err := os.Executable()
	if err != nil {
		panic(err)
	}

	if !isInteractive() {
		if pkexec, err := exec.LookPath("pkexec"); err == nil {
			args := append([]string{"pkexec", current}, os.Args[1:]...)
			if err := unix.Exec(pkexec, args, os.Environ()); err != nil {
				panic(err)
			}
		}
	}

	env := []string{"sudo"}
	for _, v := range forwardEnv {
		val, found := os.LookupEnv(v)
		if !found {
			continue
		}

		env = append(env, v+"="+val)
	}

	args := append(env, current)
	args = append(args, os.Args[1:]...)

	if err := unix.Exec("/usr/bin/sudo", args, os.Environ()); err != nil {
		panic(err)
	}
}
