// um env is a feature for ultramarine bootc specifically that lets you build local derivations "transactionally"
//
// it works similar to what you would do with `cargo add` or `pnpm add` would do, as in it:
//
// - adds the desired system package into a manifest file (`environment.toml`)
// - rebuilds the system derivation with the changes using Podman
// - optionally, attempt to apply the changes to the running system (unstable, may not work depending on the system's state)
// - use `bootc switch` to apply the update transactionally, marking the new local build as the next default
//
// Requires Podman to be already installed on the base image, or installed temporarily on the ephemeral environment (via `bootc usr-overlay`)

package main

// import (
// 	"fmt"

// 	"github.com/charmbracelet/huh"
// 	"github.com/urfave/cli/v2"
// 	"github.com/BurntSushi/toml"
// )
//

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Ultramarine-Linux/um/env"
	"github.com/Ultramarine-Linux/um/pkg/util"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

// todo: don't just print image name lol
func envStatus(c *cli.Context) error {
	image, err := env.GetBootcImage()
	if err != nil {
		return err
	}

	fmt.Println("Bootc image:", image)
	return nil
}

func envApplyChanges(c *cli.Context) error {
	fmt.Println("Applying changes...")

	manifest, err := env.LoadEnvManifest()
	if err != nil {
		return err
	}

	err = manifest.ApplyChanges()
	if err != nil {
		return err
	}

	fmt.Println("Changes applied successfully.")

	return nil
}

// initializes a new environment by creating an environment.toml and a template Containerfile
func envInit(c *cli.Context) error {
	var confirmed bool

	baseImage, err := env.GetBootcImage()
	if err != nil {
		return err
	}

	if err := huh.NewConfirm().
		Title("Initialize the local bootc derivation?").
		Description(fmt.Sprintf("This will create environment.toml and a template Containerfile based on `%s` at `%s`."+
			"\n\n"+
			"The system bootc image will be switched to `%s` and updates must now be managed via `um env update`.", baseImage, env.UmEnvContext, env.UmEnvManagedImage)).
		Affirmative("Initialize").
		Negative("Cancel").
		Value(&confirmed).
		Run(); err != nil {
		return err
	}

	if !confirmed {
		fmt.Println("Aborting...")
		return nil
	}

	if err := env.InitEnvironment(baseImage); err != nil {
		return err
	}

	fmt.Println("Initialized environment in", env.UmEnvContext)

	fmt.Println("Building initial derivation...")

	if err := envBuild(c); err != nil {
		return err
	}

	fmt.Println("Initial derivation built successfully, switching to um managed image...")

	if err := env.EnvBootcSwitch(); err != nil {
		return err
	}

	return nil
}

func envBuild(c *cli.Context) error {
	util.SudoIfNeeded([]string{})
	manifest, err := env.LoadEnvManifest()
	if err != nil {
		return err
	}

	if err := manifest.BuildContainerfile(); err != nil {
		return err
	}

	fmt.Println("Containerfile built successfully.")

	return nil
}

func handleApplyLive(c *cli.Context) error {
	var applyLive bool
	if c.Bool("apply-live") {
		applyLive = true
	}

	if applyLive {
		fmt.Println("Applying changes live...")

		// enable bootc usr-overlay

		if err := exec.Command("bootc", "usr-overlay").Run(); err != nil {
			return err
		}

		if err := envApplyChanges(c); err != nil {
			return err
		}
	}

	return nil
}

// add a package to the environment
func envAddPackage(c *cli.Context) error {
	util.SudoIfNeeded([]string{})
	manifest, err := env.LoadEnvManifest()
	if err != nil {
		return err
	}

	for _, pkg := range c.Args().Slice() {
		if manifest.AddPackage(pkg) {
			fmt.Println("Adding package:", pkg)
		} else {
			fmt.Println("Package already exists:", pkg)
		}
	}

	if err := manifest.Save(); err != nil {
		return err
	}

	if err := handleApplyLive(c); err != nil {
		return err
	}

	fmt.Println("Packages added successfully.")

	fmt.Println("Added packages successfully. Commit pending changes with `um env update`")

	return nil
}

func envRemovePackage(c *cli.Context) error {
	util.SudoIfNeeded([]string{})
	manifest, err := env.LoadEnvManifest()
	if err != nil {
		return err
	}

	for _, pkg := range c.Args().Slice() {
		if manifest.RemovePackage(pkg) {
			fmt.Println("Removed package from install list:", pkg)
		} else {
			fmt.Println("Adding package to removal list:", pkg)
		}
	}

	if err := manifest.Save(); err != nil {
		return err
	}

	if err := handleApplyLive(c); err != nil {
		return err
	}

	fmt.Println("Packages removed successfully.")
	fmt.Println("Committed pending changes with `um env update`")

	return nil
}

func envUpdate(c *cli.Context) error {
	util.SudoIfNeeded([]string{})

	if err := envBuild(c); err != nil {
		return err
	}

	fmt.Println("Rebuilt environment image, applying changes with bootc")

	// bootc update actually pulls straight from containers-storage,
	// so we can simply just do this instead of bootc switch again, assuming
	// um env is already initialized
	bootcCmd := exec.Command("bootc", "update")
	bootcCmd.Stdout = os.Stdout
	bootcCmd.Stderr = os.Stderr
	if err := bootcCmd.Run(); err != nil {
		return err
	}

	return nil
}
