package upgrade

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Ultramarine-Linux/um/pkg/util"
)

// nixDefaultProfileBin is where the Determinate installer puts nix. It isn't
// always on PATH: GUI sessions may not source /etc/profile.d/nix.sh, and pkexec
// resets the environment.
const nixDefaultProfileBin = "/nix/var/nix/profiles/default/bin/nix"

// determinateNixdPaths are where the Determinate installer puts determinate-nixd.
var determinateNixdPaths = []string{"/usr/local/bin/determinate-nixd"}

// findExecutable returns the first executable among paths, falling back to PATH.
func findExecutable(name string, paths ...string) string {
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func nixBinary() string { return findExecutable("nix", nixDefaultProfileBin) }

func determinateNixdBinary() string {
	return findExecutable("determinate-nixd", determinateNixdPaths...)
}

// nixProfileElement is one entry of `nix profile list --json`.
type nixProfileElement struct {
	Name        string   `json:"-"`
	Active      *bool    `json:"active"`
	AttrPath    string   `json:"attrPath"`
	OriginalURL string   `json:"originalUrl"`
	URL         string   `json:"url"`
	Outputs     []string `json:"outputs"`
	StorePaths  []string `json:"storePaths"`
}

// parseNixProfileList parses `nix profile list --json`. Version 3 (Nix 2.20+)
// keys elements by name; older versions use an array without names.
func parseNixProfileList(output []byte) ([]nixProfileElement, error) {
	var list struct {
		Version  int             `json:"version"`
		Elements json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, fmt.Errorf("unexpected nix profile output: %w", err)
	}
	if len(list.Elements) == 0 || string(list.Elements) == "null" {
		return nil, nil
	}

	var elements []nixProfileElement
	if list.Elements[0] == '{' {
		byName := map[string]nixProfileElement{}
		if err := json.Unmarshal(list.Elements, &byName); err != nil {
			return nil, fmt.Errorf("unexpected nix profile elements: %w", err)
		}
		for name, e := range byName {
			e.Name = name
			elements = append(elements, e)
		}
	} else {
		if err := json.Unmarshal(list.Elements, &elements); err != nil {
			return nil, fmt.Errorf("unexpected nix profile elements: %w", err)
		}
		for i := range elements {
			// Older profiles have no names; nix named them after the attribute.
			parts := strings.Split(elements[i].AttrPath, ".")
			elements[i].Name = parts[len(parts)-1]
		}
	}

	upgradable := elements[:0]
	for _, e := range elements {
		// Packages installed from a store path have no flake to upgrade from.
		if e.OriginalURL == "" || e.AttrPath == "" || (e.Active != nil && !*e.Active) {
			continue
		}
		upgradable = append(upgradable, e)
	}
	sort.Slice(upgradable, func(i, j int) bool { return upgradable[i].Name < upgradable[j].Name })
	return upgradable, nil
}

// nixUpdatesExpr evaluates the packages `nix profile upgrade` would install,
// without building or changing anything. Elements are passed through the
// environment so nothing has to be escaped into Nix syntax.
const nixUpdatesExpr = `
let
  elements = builtins.fromJSON (builtins.getEnv "UM_NIX_ELEMENTS");
  # Missing attributes can't be caught with tryEval, so walk the path safely.
  getPath = root: path: builtins.foldl' (a: k: if a != null && builtins.isAttrs a && a ? ${k} then a.${k} else null) root path;
  resolve = e:
    let
      flake = builtins.tryEval (builtins.getFlake e.flake);
      pkg = if flake.success then getPath flake.value e.attrPath else null;
      outputs = if e.outputs != null then e.outputs else (pkg.meta.outputsToInstall or [ "out" ]);
      outPaths = builtins.tryEval (map (o: pkg.${o}.outPath) outputs);
    in
    if pkg == null || !outPaths.success then { error = true; }
    else { error = false; name = pkg.name or ""; version = pkg.version or ""; outPaths = outPaths.value; };
in
map resolve elements
`

type nixEvalInput struct {
	Flake    string   `json:"flake"`
	AttrPath []string `json:"attrPath"`
	Outputs  []string `json:"outputs"`
}

type nixEvalResult struct {
	Error    bool     `json:"error"`
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	OutPaths []string `json:"outPaths"`
}

// storePathName returns the name part of a store path:
// /nix/store/<hash>-flox-1.17.0 -> flox-1.17.0.
func storePathName(path string) string {
	base := filepath.Base(path)
	if _, name, ok := strings.Cut(base, "-"); ok {
		return name
	}
	return base
}

// drvVersion splits off the version like builtins.parseDrvName: it starts
// after the first dash that isn't followed by a letter.
func drvVersion(name string) string {
	for i := 0; i < len(name)-1; i++ {
		if name[i] == '-' && !unicode.IsLetter(rune(name[i+1])) {
			return name[i+1:]
		}
	}
	return ""
}

// sameStorePaths reports whether two sets of store paths are equal.
func sameStorePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, p := range a {
		set[p] = true
	}
	for _, p := range b {
		if !set[p] {
			return false
		}
	}
	return true
}

// nixUpdateFor turns an element and its evaluated upgrade into an update, or
// returns false if the upgrade would install exactly what is installed now.
func nixUpdateFor(e nixProfileElement, r nixEvalResult) (PackageUpdate, bool) {
	if r.Error || len(r.OutPaths) == 0 || sameStorePaths(e.StorePaths, r.OutPaths) {
		return PackageUpdate{}, false
	}
	oldVersion := ""
	if len(e.StorePaths) > 0 {
		oldVersion = drvVersion(storePathName(e.StorePaths[0]))
	}
	newVersion := r.Version
	if newVersion == "" {
		newVersion = drvVersion(r.Name)
	}
	return PackageUpdate{
		ID: "nix:" + e.Name, Name: e.Name, Source: "Nix (" + e.OriginalURL + ")",
		Backend: "nix", Kind: KindPackage, Icon: "package-x-generic",
		OldVersion: oldVersion, NewVersion: newVersion,
	}, true
}

// nixAppFiles lists the app files a package installs in its store path, for
// the same app detection as RPMs.
func nixAppFiles(storePaths []string) *rpmAppFiles {
	files := &rpmAppFiles{}
	for _, storePath := range storePaths {
		share := filepath.Join(storePath, "share")
		for _, dir := range []string{"metainfo", "appdata"} {
			matches, _ := filepath.Glob(filepath.Join(share, dir, "*.xml"))
			files.Metainfo = append(files.Metainfo, matches...)
		}
		matches, _ := filepath.Glob(filepath.Join(share, "applications", "*.desktop"))
		files.Desktop = append(files.Desktop, matches...)
		if files.ApplicationsDir == "" && len(matches) > 0 {
			files.ApplicationsDir = filepath.Join(share, "applications")
		}
	}
	return files
}

// canManageNixProfile reports whether um should touch a Nix profile: nix is
// installed and um runs as a regular user (profiles are per-user).
func canManageNixProfile() bool {
	return nixBinary() != "" && os.Getuid() != 0
}

func nixPackageUpdates() ([]PackageUpdate, error) {
	nix := nixBinary()
	output, err := exec.Command(nix, "profile", "list", "--json").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("nix profile list failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	elements, err := parseNixProfileList(output)
	if err != nil || len(elements) == 0 {
		return nil, err
	}

	inputs := make([]nixEvalInput, 0, len(elements))
	for _, e := range elements {
		inputs = append(inputs, nixEvalInput{Flake: e.OriginalURL, AttrPath: strings.Split(e.AttrPath, "."), Outputs: e.Outputs})
	}
	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, err
	}

	// --impure is needed to resolve unlocked flake references; nix's
	// tarball-ttl decides when they are fetched again, like dnf's metadata expiry.
	cmd := exec.Command(nix, "eval", "--json", "--impure", "--expr", nixUpdatesExpr)
	cmd.Env = append(os.Environ(), "UM_NIX_ELEMENTS="+string(inputJSON))
	evalOutput, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("evaluating nix profile upgrades failed: %s", lastLine(string(exitErr.Stderr)))
		}
		return nil, err
	}
	var results []nixEvalResult
	if err := json.Unmarshal(evalOutput, &results); err != nil || len(results) != len(elements) {
		return nil, fmt.Errorf("unexpected nix eval output")
	}

	var updates []PackageUpdate
	for i, e := range elements {
		update, ok := nixUpdateFor(e, results[i])
		if !ok {
			continue
		}
		if app, ok := resolveApp(e.Name, nixAppFiles(e.StorePaths), os.ReadFile); ok {
			update.Kind = KindApp
			update.AppID = app.ID
			update.Name = app.Name
			if app.Icon != "" {
				update.Icon = app.Icon
			}
		}
		updates = append(updates, update)
	}
	return updates, nil
}

// lastLine returns the last non-empty line of s (nix puts the error last).
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// upgradeNixProfile upgrades the user's profile. Flake nixConfig (such as
// extra substituters) is not accepted automatically, same as nix itself.
func upgradeNixProfile() error {
	return util.RunCommand(nixBinary(), "profile", "upgrade", "--all")
}

// determinateNixVersions parses `determinate-nixd version`. latest is empty if
// the output doesn't say (e.g. the version check couldn't reach the network).
func determinateNixVersions(output string) (installed, latest string) {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Determinate Nixd client version":
			installed = value
		case "Latest version":
			latest = value
		}
	}
	return installed, latest
}

// determinateNixUpdate reports an update of Determinate Nix itself, if
// determinate-nixd is installed and says a newer version is available.
func determinateNixUpdate() ([]PackageUpdate, error) {
	nixd := determinateNixdBinary()
	if nixd == "" {
		return nil, nil
	}
	output, err := exec.Command(nixd, "version").Output()
	if err != nil {
		return nil, fmt.Errorf("determinate-nixd version failed: %w", err)
	}
	installed, latest := determinateNixVersions(string(output))
	if installed == "" || latest == "" || installed == latest {
		return nil, nil
	}
	return []PackageUpdate{{
		ID: "nix:determinate-nix", Name: "Determinate Nix", Source: "Nix (Determinate Systems)",
		Backend: "nix", Kind: KindPackage, Icon: "package-x-generic",
		OldVersion: installed, NewVersion: latest,
	}}, nil
}

// upgradeDeterminateNix upgrades Nix itself. Needs root.
func upgradeDeterminateNix() error {
	nixd := determinateNixdBinary()
	if nixd == "" {
		return nil
	}
	return util.RunCommand(nixd, "upgrade")
}
