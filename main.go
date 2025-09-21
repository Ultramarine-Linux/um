package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

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
							&cli.BoolFlag{
								Name:    "yes",
								Aliases: []string{"y"},
								Usage:   "Automatically confirm enabling tweaks without prompting",
							},
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
							&cli.BoolFlag{
								Name:    "yes",
								Aliases: []string{"y"},
								Usage:   "Automatically confirm disabling tweaks without prompting",
							},
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
