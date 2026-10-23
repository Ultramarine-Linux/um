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
//
// # Automatic updates and Apply Tonight
//
// [Settings] (in /etc/um/upgrade.json) decide whether updates install by
// themselves ("Let your device decide") or only notify ("Ask me"), how long
// the device must be unused first, an optional time window, and whether RPM
// updates are staged for the next restart.
//
// um's timers (systemd/ in the repository) call [RunScheduled] (system, as
// root) and [RunUser] (per user, started by the XDG autostart entry in
// xdg/) every few minutes. They track how long no one has used the device
// and install at most once a day; otherwise they do nothing, quietly. With automatic updates off, RunUser notifies about
// pending updates once a day instead.
//
// [ScheduleTonight] ("Apply Tonight") creates transient timers that do the
// same once, tonight, even if automatic updates are off. um never restarts
// the device by itself.
package upgrade
