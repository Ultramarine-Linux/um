package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

const envPMHelperCategory string = "Package management helpers"

var envApplyLiveFlag = &cli.BoolFlag{
	Name:  "apply-live",
	Usage: "Apply package changes live with bootc usr-overlay",
}

var yesFlag = &cli.BoolFlag{
	Name:    "yes",
	Aliases: []string{"y"},
	Usage:   "Automatically confirm without prompting",
}

// TODO: pls refactor all of this mess

func runCli() error {
	app := &cli.App{
		Name:  "um",
		Usage: "manage an Ultramarine Linux system",
		Commands: []*cli.Command{
			{
				Name:   "status",
				Usage:  "Display the status of the system",
				Action: status,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "upload",
						Usage: "Upload status to boba",
					},
					&cli.BoolFlag{
						Name:  "json",
						Usage: "Output system status in JSON",
					},
				},
			},
			{
				Name:   "tweaks",
				Usage:  "Manage Ultramarine tweaks, a set of optional system patches and configurations",
				Action: listTweaks,
				Subcommands: []*cli.Command{
					{
						Name:    "enable",
						Usage:   "Enable an Ultramarine tweak",
						Aliases: []string{"en", "apply", "set"},
						Description: "This will run an Ansible playbook to apply the tweak. " +
							"Tweaks can be of type 'oneshot' or 'toggle'. 'oneshot' tweaks can only be enabled once, while 'toggle' tweaks can also be disabled.",
						Action: enableTweak,
						Flags: []cli.Flag{
							yesFlag,
						},
					},
					{
						Name:    "disable",
						Aliases: []string{"dis", "remove", "unset"},
						Usage:   "Disable an Ultramarine tweak",
						Description: "This will run an Ansible playbook to revert the tweak. " +
							"Only tweaks of type 'toggle' can be disabled. 'oneshot' tweaks cannot be reverted.",
						Action: disableTweak,
						Flags: []cli.Flag{
							yesFlag,
						},
					},
					{
						Name:    "run",
						Usage:   "Run an Ultramarine tweak script",
						Aliases: []string{"exec", "execute"},
						Description: "This will run a script tweak. " +
							"Script tweaks are one-time scripts that perform a specific action.",
						Action: runTweak,
					},
					{
						Name:   "list",
						Usage:  "List all available Ultramarine tweaks",
						Action: listTweaks,
					},
				},
			},

			{
				Name:  "env",
				Usage: "Manage local bootc derivations",
				Subcommands: []*cli.Command{
					{
						Name:   "init",
						Usage:  "Create a bootc environment manifest and containerfile",
						Action: envInit,
						Flags: []cli.Flag{
							yesFlag,
						},
					},
					{
						Name:   "build",
						Usage:  "Build the local derivation from a Containerfile",
						Aliases: []string{"b"},
						Action: envBuild,
					},
					{
						Name:     "add",
						Usage:    "Add a package to the environment",
						Action:   envAddPackage,
						Category: envPMHelperCategory,
						Aliases: []string{"install", "i", "a", "in"},
						Flags: []cli.Flag{
							envApplyLiveFlag,
						},
					},
					{
						Name:     "remove",
						Usage:    "Remove a package from the environment",
						Action:   envRemovePackage,
						Aliases: []string{"uninstall", "rm", "r"},
						Category: envPMHelperCategory,
						Flags: []cli.Flag{
							envApplyLiveFlag,
						},
					},
					{
						Name:   "apply-changes",
						Usage:  "Apply pending changes to the bootc environment",
						Action: envApplyChanges,
					},
					{
						Name:   "update",
						Usage:  "Update the base image and rebuild the environment",
						Action: envUpdate,
					},
				},
			},

			// {
			// 	Name:   "experiments",
			// 	Usage:  "manage Ultramarine Linux experiments, a preview of features to come",
			// 	Action: listExperiments,
			// 	Subcommands: []*cli.Command{
			// 		{
			// 			Name:   "enable",
			// 			Action: enableExperiment,
			// 		},
			// 		{
			// 			Name:   "disable",
			// 			Action: disableExperiment,
			// 		},
			// 	},
			// },
		},
	}

	if err := app.Run(os.Args); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := runCli(); err != nil {
		log.Fatal(err)
	}
}
