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
				Usage:  "display the status of the system",
				Action: status,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "json",
						Usage: "Output system status in JSON",
					},
				},
			},
			{
				Name:   "experiments",
				Usage:  "manage Ultramarine Linux experiments, a preview of features to come",
				Action: listExperiments,
				Subcommands: []*cli.Command{
					{
						Name:   "enable",
						Action: enableExperiment,
					},
					{
						Name:   "disable",
						Action: disableExperiment,
					},
				},
			},
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
