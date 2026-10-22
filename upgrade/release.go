package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/acobaugh/osrelease"
)

type collectionsResponse struct {
	Collections []collection `json:"collections"`
}

type collection struct {
	AllowRetire bool   `json:"allow_retire"`
	Branchname  string `json:"branchname"`
	DateCreated string `json:"date_created"`
	DateUpdated string `json:"date_updated"`
	DistTag     string `json:"dist_tag"`
	KojiName    string `json:"koji_name"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Version     string `json:"version"`
}

// CurrentReleaseVersion reads /etc/os-release to safely discover what version the user is running
func CurrentReleaseVersion() (string, error) {
	release, err := osrelease.Read()
	if err != nil {
		return "", err
	}
	return release["VERSION_ID"], nil
}

// NextReleaseVersion calculates the next upgrade jump.
func NextReleaseVersion(currentVersion string) (string, error) {
	const url = "https://ultramarine-linux.org/pkgdb/collections.json"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("fetch collections.json: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var cr collectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}

	// Build a sorted list of active numeric versions.
	type vitem struct {
		raw string
		n   int
	}
	var active []vitem
	for _, c := range cr.Collections {
		if strings.TrimSpace(c.Status) != "Active" {
			continue
		}
		n, ok := parseMajorVersionInt(c.Version)
		if !ok {
			continue // skip "devel" or other non-numeric versions
		}
		active = append(active, vitem{raw: c.Version, n: n})
	}
	if len(active) == 0 {
		return "", errors.New("no Active numeric versions found in collections.json")
	}

	sort.Slice(active, func(i, j int) bool { return active[i].n < active[j].n })

	curN, curIsNum := parseMajorVersionInt(currentVersion)

	if curN == 0 {
		return "", fmt.Errorf("Cannot detect current version from /etc/os-release (VERSION_ID=%q)", currentVersion)
	}

	// If currentVersion is not numeric (e.g., "devel"), treat it as "before everything".
	if !curIsNum {
		return strconv.Itoa(active[0].n), nil
	}

	// Find the first Active version with n > current.
	for i := 0; i < len(active); i++ {
		if active[i].n > curN {
			return strconv.Itoa(active[i].n), nil
		}
	}

	return "", fmt.Errorf("no next Active version found greater than %q", currentVersion)
}

func parseMajorVersionInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Versions in the JSON are like "42", "43", "44". Keep it strict.
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
