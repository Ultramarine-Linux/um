// Package upgrade lists and installs updates for an Ultramarine system:
// RPM packages (dnf), Flatpaks (system and user installations) and Nix
// (Determinate Nix itself and the user's `nix profile`).
//
// # Listing
//
// [List] collects pending updates from every available source. Each update
// says whether it's a desktop app ([KindApp]), a runtime or another package,
// so frontends like the Plasma Updates KCM can show apps first. RPM apps are
// recognized from their AppStream metainfo, or their launcher if they ship no
// metainfo. A failing source is reported in [ListResponse.Errors] without
// hiding the others.
//
// # Installing
//
// [ApplyUser] updates what belongs to the invoking user and must run as that
// user; [ApplySystem] updates the rest and needs root.
//
// # Nix
//
// Nix support follows how Ultramarine's `nix` tweak installs Nix: the
// Determinate Systems installer, i.e. a multi-user Determinate Nix with its
// binaries in the default profile and determinate-nixd managing Nix itself.
// nix-env, channels and home-manager are not handled.
package upgrade
