package util

import (
	"os"

	"golang.org/x/sys/unix"
)

func SudoIfNeeded(forwardEnv []string) {
	if os.Getuid() == 0 {
		return
	}

	env := []string{"f"} // don't ask
	for _, v := range forwardEnv {
		val, found := os.LookupEnv(v)
		if !found {
			continue
		}

		env = append(env, v+"="+val)
	}

	current, err := os.Executable()
	if err != nil {
		panic(err)
	}

	println(current)

	args := append(env, current)
	args = append(args, os.Args[1:]...)

	if err := unix.Exec("/usr/bin/sudo", args, os.Environ()); err != nil {
		panic(err)
	}
}
