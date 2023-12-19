package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

// TODO: pls refactor all of this mess

func main() {
	app := &cli.App{
		Name:  "um",
		Usage: "manage an Ultramarine Linux system",
		Commands: []*cli.Command{
			{
				Name:   "status",
				Usage:  "display the status of the system",
				Action: status,
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
		log.Fatal(err)
	}
}
