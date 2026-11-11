package main

import (
	"errors"
	"os/exec"
	"strconv"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestPassThroughExitCode(t *testing.T) {
	for _, tc := range []struct {
		code, want int
	}{
		{3, 3},
		{126, 1},
		{127, 1},
		{-1, 1},
	} {
		script := "exit " + strconv.Itoa(tc.code)
		if tc.code < 0 {
			script = "kill -TERM $$"
		}
		err := passThroughExitCode("script.sh", exec.Command("/bin/sh", "-c", script).Run())
		var coder cli.ExitCoder
		if !errors.As(err, &coder) || coder.ExitCode() != tc.want {
			t.Errorf("exit %d: got %v, want exit code %d", tc.code, err, tc.want)
		}
	}
	if err := passThroughExitCode("script.sh", nil); err != nil {
		t.Errorf("success should stay nil, got %v", err)
	}
}

func TestTweakIDArg(t *testing.T) {
	run := func(args ...string) (id string, err error) {
		app := &cli.App{
			Commands: []*cli.Command{{
				Name:  "enable",
				Flags: []cli.Flag{yesFlag},
				Action: func(c *cli.Context) error {
					id, err = tweakIDArg(c)
					return nil
				},
			}},
		}
		if runErr := app.Run(append([]string{"um", "enable"}, args...)); runErr != nil {
			t.Fatal(runErr)
		}
		return id, err
	}

	if id, err := run("--yes", "nix"); err != nil || id != "nix" {
		t.Errorf("got %q, %v", id, err)
	}
	for _, args := range [][]string{{}, {"nix", "--yes"}, {"a", "b"}, {"../x"}} {
		if _, err := run(args...); err == nil {
			t.Errorf("%q should be rejected", args)
		}
	}
}
