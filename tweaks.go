package main

import (
	"encoding/json"
	"fmt"
	"os"

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

func listTweaks(c *cli.Context) error {
	// Listing only reads metadata and world-readable state, so no escalation
	// is needed. This lets GUI frontends query tweaks without a password prompt.
	tweakList, err := tweaks.List()
	if err != nil {
		return err
	}

	entries := tweaks.SortedEntries(tweakList)

	if c.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(struct {
			Tweaks []tweaks.ListEntry `json:"tweaks"`
		}{entries})
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
		if tweak.Type != "script" {
			fmt.Printf("  Enabled: %t\n", tweak.Enabled)
		}
		fmt.Println()
	}

	return nil
}

func enableTweak(c *cli.Context) error {
	util.SudoIfNeeded(Envars)
	if c.Args().Len() < 1 {
		return fmt.Errorf("Please provide a tweak ID to enable")
	}
	tweakID := c.Args().Get(0)

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
	// Confirm with the user if the tweak is provided but not stable or has a warning
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

	// huh.Println("Enabling tweak:",
	if !confirm {
		fmt.Println("Aborting...")
		return nil
	}

	if err := tweak.Enable(); err != nil {
		return err
	}

	return nil
}

func disableTweak(c *cli.Context) error {
	util.SudoIfNeeded(Envars)
	if c.Args().Len() < 1 {
		return fmt.Errorf("Please provide a tweak ID to disable")
	}
	tweakID := c.Args().Get(0)

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
		return err
	}

	return nil
}

func runTweak(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("A tweak ID must be passed", 1)
	}
	tweakID := c.Args().First()

	util.SudoIfNeeded(Envars)

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

	return tweak.Run()
}
