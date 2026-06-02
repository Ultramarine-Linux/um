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

	"github.com/Ultramarine-Linux/um/env"
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
