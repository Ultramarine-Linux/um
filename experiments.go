package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Ultramarine-Linux/um/experiments"
	"github.com/Ultramarine-Linux/um/util"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
)

const (
	purple    = lipgloss.Color("99")
	gray      = lipgloss.Color("245")
	lightGray = lipgloss.Color("241")
	red       = lipgloss.Color("9")
	lightRed  = lipgloss.Color("1")
	yellow    = lipgloss.Color("11")
	orange    = lipgloss.Color("208")
)

func experimentsToRows(exps []experiments.Experiment) [][]string {
	return lo.Map(exps, func(exp experiments.Experiment, index int) []string {
		enabled := "False"
		if exp.Enabled {
			enabled = "True"
		}
		return []string{exp.Id, exp.Name, exp.Description, exp.Stability.String(), enabled}
	})
}

func listExperiments(c *cli.Context) error {
	util.SudoIfNeeded([]string{"UM_DATA"})

	exps, err := experiments.List()
	if err != nil {
		return err
	}

	re := lipgloss.NewRenderer(os.Stdout)

	var (
		// HeaderStyle is the lipgloss style used for the table headers.
		HeaderStyle = re.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center).Padding(0, 1)
		// CellStyle is the base lipgloss style used for the table rows.
		CellStyle = re.NewStyle().Padding(0, 1)
		// OddRowStyle is the lipgloss style used for odd-numbered table rows.
		OddRowStyle = CellStyle.Copy().Foreground(gray)
		// EvenRowStyle is the lipgloss style used for even-numbered table rows.
		EvenRowStyle = CellStyle.Copy().Foreground(lightGray)
		// BorderStyle is the lipgloss style used for the table border.
		GFLStyle   = CellStyle.Copy().Foreground(red)
		DevelStyle = CellStyle.Copy().Foreground(orange)
		AlphaStyle = CellStyle.Copy().Foreground(yellow)
		BetaStyle  = CellStyle.Copy().Foreground(gray)
	)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row != 0 && col == 3 {
				exp := exps[row-1]
				switch exp.Stability {
				case experiments.GFL:
					return GFLStyle
				case experiments.Devel:
					return DevelStyle
				case experiments.Alpha:
					return AlphaStyle
				case experiments.Beta:
					return BetaStyle
				}
			}

			switch {
			case row == 0:
				return HeaderStyle
			case row%2 == 0:
				return EvenRowStyle
			default:
				return OddRowStyle
			}
		}).
		Headers("ID", "Name", "Description", "Stability", "Enabled").
		Rows(experimentsToRows(exps)...)

	fmt.Print(t)

	return err
}

func enableExperiment(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("An experiment id must be passed", 1)
	}

	util.SudoIfNeeded([]string{"UM_DATA"})

	exp, err := experiments.Find(c.Args().First())
	if err != nil {
		return err
	}

	if exp == nil {
		return cli.Exit("The experiment id passed is invalid", 1)
	}

	if exp.Enabled {
		return cli.Exit("This experiment is already enabled", 1)
	}

	var confirmed bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Enable the \"%s\" experiment?", exp.Name)).
		Description(fmt.Sprintf("%s\n\nExperiments are intended to provide an as-is preview of potentially upcoming features.\nThey are unstable and may cause irreparable damage to your system.", exp.Description)).
		Affirmative("Enable").
		Negative("Cancel").
		Value(&confirmed).Run(); err != nil {
		return err
	}

	if !confirmed {
		fmt.Println("Goodbye! No experiments enabled.")
		return nil
	}

	if err := huh.NewConfirm().
		Title("Are you REALLY sure?").
		Description("Make sure your data is backed up.\nWe might not be able to help you if something goes wrong.").
		Affirmative("Enable").
		Negative("Cancel").
		Value(&confirmed).Run(); err != nil {
		return err
	}

	if !confirmed {
		fmt.Println("Goodbye! No experiments enabled.")
		return nil
	}

	if exp.Stability == experiments.GFL {
		if err := huh.NewConfirm().
			Title("Are you REALLY REALLY sure?").
			Description("This selected experiment has a marked stability of GFL. It is KNOWN to cause system breakage.\nUnless you're a contributor hacking on Ultramarine, you really shouldn't enable this.").
			Affirmative("Enable").
			Negative("Cancel").
			Value(&confirmed).Run(); err != nil {
			return err
		}

		if !confirmed {
			fmt.Println("Goodbye! No experiments enabled.")
			return nil
		}
	}

	fmt.Println("Please enter your password if prompted to do so.")
	fmt.Println("Running the experiment's up script ::")

	if err := experiments.MarkEnabled(exp.Id, true); err != nil {
		return err
	}

	cmd := exec.Command(exp.UpScript)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func disableExperiment(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("An experiment id must be passed", 1)
	}

	util.SudoIfNeeded([]string{"UM_DATA"})

	exp, err := experiments.Find(c.Args().First())
	if err != nil {
		return err
	}

	if exp == nil {
		return cli.Exit("The experiment id passed is invalid", 1)
	}

	if !exp.Enabled {
		return cli.Exit("This experiment isn't ealread enabled", 1)
	}

	var confirmed bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Disable the \"%s\" experiment?", exp.Name)).
		Description("Disabling an experiment will make a best-effort attempt to \"reset\" your system to before it.\nHowever, not all data might get cleaned up.").
		Affirmative("Disable").
		Negative("Cancel").
		Value(&confirmed).Run(); err != nil {
		return err
	}

	if !confirmed {
		fmt.Println("Goodbye! No experiments disabled.")
		return nil
	}

	fmt.Println("Please enter your password if prompted to do so.")
	fmt.Println("Running the experiment's down script ::")

	if err := experiments.MarkEnabled(exp.Id, false); err != nil {
		return err
	}

	cmd := exec.Command(exp.DownScript)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
