package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/Ultramarine-Linux/um/tweaks"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

var Envars = []string{
	"UM_TWEAKS_PATH",
	"UM_DATA",
	"UM_STATE",
}

func passThroughExitCode(what string, err error) error {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	code := exitErr.ExitCode()
	// 126/127 mean pkexec auth failure to frontends, and a signal kill (-1) would become 255
	if code <= 0 || code == 126 || code == 127 {
		code = 1
	}
	return cli.Exit(fmt.Sprintf("%s failed: %v", what, err), code)
}

func tweakIDArg(c *cli.Context) (string, error) {
	if c.NArg() == 0 {
		return "", cli.Exit("Please provide a tweak ID", 1)
	}
	if c.NArg() > 1 {
		return "", cli.Exit(fmt.Sprintf("Unexpected argument %q. Flags must come before the tweak ID", c.Args().Get(1)), 1)
	}
	id := c.Args().First()
	if err := tweaks.ValidateID(id); err != nil {
		return "", err
	}
	return id, nil
}

func escalateForTweak(c *cli.Context, needsConfirmation func(tweakID string) bool) (string, error) {
	tweakID, err := tweakIDArg(c)
	if err != nil {
		return "", err
	}
	if !c.Bool("yes") && !util.CanPrompt() && needsConfirmation(tweakID) {
		return "", cli.Exit("Not running in a terminal, pass --yes to confirm", 1)
	}
	return tweakID, util.SudoIfNeeded(Envars)
}

func alwaysConfirm(string) bool { return true }

func hasWarning(tweakID string) bool {
	tweak, err := tweaks.LoadTweakId(tweakID)
	return err == nil && tweak.Warning != nil
}

func listTweaks(c *cli.Context) error {
	if c.NArg() > 0 {
		return cli.Exit(fmt.Sprintf("Unexpected argument %q. Flags must come before arguments", c.Args().First()), 1)
	}
	tweakList, err := tweaks.List()
	if err != nil {
		return err
	}

	entries := tweaks.SortedEntries(tweakList)

	if c.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(struct {
			Version int                `json:"version"`
			Tweaks  []tweaks.ListEntry `json:"tweaks"`
		}{1, entries})
	}

	for _, tweak := range entries {
		fmt.Printf("- ID: %s\n", tweak.ID)
		if tweak.Description != "" {
			fmt.Printf("  Description: %s\n", tweak.Description)
		}
		fmt.Printf("  Type: %s\n", tweak.Type)
		if tweak.Stability != "" {
			fmt.Printf("  Stability: %s\n", tweak.Stability)
		}
		if tweak.StateError != "" {
			fmt.Printf("  Enabled: unknown (%s)\n", tweak.StateError)
		} else if tweak.Type != "script" {
			fmt.Printf("  Enabled: %t\n", tweak.Enabled)
		}
		fmt.Println()
	}

	return nil
}

func enableTweak(c *cli.Context) error {
	tweakID, err := escalateForTweak(c, alwaysConfirm)
	if err != nil {
		return err
	}

	tweak, err := tweaks.LoadTweakId(tweakID)
	if err != nil {
		return err
	}

	switch tweak.TweakType {
	case tweaks.TweakTypeOneshot:
	case tweaks.TweakTypeToggle:
	default:
		return tweaks.TweakTypeNotSupportedError(tweak.TweakType)

	}

	yesFlag := c.Bool("yes")

	var confirm bool
	if yesFlag {
		confirm = true
	} else {
		var description string
		if tweak.Warning != nil {
			description = *tweak.Warning
		} else if tweak.Stability != nil && *tweak.Stability != tweaks.Stable {
			description += fmt.Sprintf("This tweak is marked as '%s' and may be unstable.\n\n", tweak.Stability.String())
		}

		err := huh.NewConfirm().
			Title("Would you like to enable this tweak? (" + tweakID + ")").
			Affirmative("Enable").
			Negative("Cancel").
			Description(description).
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}
	}

	if !confirm {
		fmt.Println("Aborting...")
		return nil
	}

	if err := tweak.Enable(); err != nil {
		return passThroughExitCode("ansible-playbook", err)
	}

	return nil
}

func disableTweak(c *cli.Context) error {
	tweakID, err := escalateForTweak(c, alwaysConfirm)
	if err != nil {
		return err
	}

	tweak, err := tweaks.LoadTweakId(tweakID)
	if err != nil {
		return err
	}

	if tweak.TweakType != tweaks.TweakTypeToggle {
		return tweaks.TweakTypeNotSupportedError(tweak.TweakType)
	}
	yesFlag := c.Bool("yes")

	var confirm bool
	if yesFlag {
		confirm = true
	} else {
		err = huh.NewConfirm().
			Title("Would you like to disable this tweak? (" + tweakID + ")").
			Affirmative("Disable").
			Negative("Cancel").
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}
	}

	if !confirm {
		fmt.Println("Aborting...")
		return nil
	}

	if err := tweak.Disable(); err != nil {
		return passThroughExitCode("ansible-playbook", err)
	}

	return nil
}

func runTweak(c *cli.Context) error {
	tweakID, err := escalateForTweak(c, hasWarning)
	if err != nil {
		return err
	}

	tweak, err := tweaks.LoadTweakId(tweakID)
	if err != nil {
		return err
	}

	if tweak == nil {
		return cli.Exit("The tweak id passed is invalid", 1)
	}

	if tweak.TweakType != tweaks.TweakTypeScript {
		return tweaks.TweakTypeNotSupportedError(tweak.TweakType)
	}
	yesFlag := c.Bool("yes")

	var confirm bool
	if yesFlag {
		confirm = true
	} else if tweak.Warning != nil {
		err = huh.NewConfirm().
			Title("Would you like to run this tweak? (" + tweakID + ")").
			Affirmative("Run").
			Negative("Cancel").
			Description(*tweak.Warning).
			Value(&confirm).
			Run()
		if err != nil {
			return err
		}
	} else {
		confirm = true
	}

	if !confirm {
		fmt.Println("Aborting...")
		return nil
	}

	return passThroughExitCode("script.sh", tweak.Run())
}
