# um

The Ultramarine CLI. More information can be found on our wiki: https://wiki.ultramarine-linux.org/en/usage/umcli/

## 📦 Dependencies

```
flatpak
rpm
ansible
```

### 🛠️ Build Dependencies

```
go
flatpak-devel
rpm-devel
```

## Upgrades

`um upgrade` upgrades the system's RPM and Flatpak packages, and the invoking user's Nix profile, in one operation, similar to topgrade.

`um upgrade list --json` exposes a structured update inventory for desktop integrations such as the Plasma Updates KCM. Each entry includes `id`, `name`, `source`, `backend` (`rpm`, `flatpak` or `nix`), `kind`, `old_version`, `new_version`, `icon`, `download_size` and `installed_size_change` (bytes, 0 if unknown; Flatpak reports full sizes, so deltas can download less) and, for apps, `app_id`, for both RPM and Flatpak (system and user installation) updates.

`kind` is `app` for desktop applications, `runtime` for Flatpak runtimes and extensions, and `package` for everything else, so frontends can list apps first. Flatpak apps are recognized by their `app/` ref. An RPM is an app if it ships AppStream metainfo (`/usr/share/metainfo`) describing a `desktop-application`; `name` and `app_id` then come from the metainfo, and `icon` from the metainfo or its launcher. RPMs with no metainfo at all, as is common for third-party packages, count as apps if they install a visible launcher in `/usr/share/applications`. `id` is unique: `name.arch` for RPMs, `app.id//branch` for Flatpaks (suffixed with `@user` for the user installation). Sources that fail to be queried are reported in `errors` while updates from the remaining sources are still listed; the command only exits non-zero when no source could be queried.

Nix support follows how the `nix` tweak installs Nix (the Determinate Systems installer). `um` calls `/nix/var/nix/profiles/default/bin/nix` and `/usr/local/bin/determinate-nixd` directly, since GUI sessions and `pkexec` don't always have them on `PATH`. If Nix isn't installed, it's skipped.

- **Determinate Nix itself:** listed when `determinate-nixd version` reports a newer version. `apply` runs `determinate-nixd upgrade` after escalating.
- **The user's `nix profile`:** packages installed from a flake. Listing evaluates what `nix profile upgrade` would install (one `nix eval --impure`, nothing is built or changed) and reports packages whose store paths would change. Unlocked flake references are re-fetched according to Nix's `tarball-ttl`. Packages that ship AppStream metainfo or a launcher are reported as apps. Profiles are per-user, so this part is skipped when `um` runs as root. `apply` runs `nix profile upgrade --all` before escalating, and doesn't accept flake `nixConfig` (such as extra substituters) on its own. `nix-env`, channels and home-manager aren't handled.

Without `--yes`, `dnf` and `flatpak` prompt for confirmation as usual. User-installation Flatpaks and the Nix profile are updated before escalating to root. Use `um upgrade apply --yes` explicitly when scripting or integrating the operation. The `--yes` flag is also available on the top-level `um upgrade` command.

### Automatic updates and Apply Tonight

`um upgrade settings` shows the system-wide settings in `/etc/um/upgrade.json`; `--json` is for frontends. Changing them needs root:

| flag | default | meaning |
| --- | --- | --- |
| `--automatic=true\|false` | `true` | install updates by themselves ("Let your device decide"), or only notify ("Ask me") |
| `--inactive-minutes=N` | `30` | install once no one has used the device for this long |
| `--window=HH:MM-HH:MM` | empty | only install during these hours (may wrap past midnight); empty means any time |
| `--offline=true\|false` | `false` | stage RPM updates as a dnf offline transaction for the next restart instead of installing them live |

A device counts as in use while any login session is active and unlocked (and not idle). `um-upgrade-automatic.timer` (system) and `um-upgrade-user.timer` (user) check every 5 minutes. They track how long the device has been unused, and install system updates and each user's own updates (user Flatpaks, Nix profile) once that exceeds `inactive-minutes`, inside the window, on AC power, at most once a day. Checks that don't install anything don't log anything. `um` never restarts the device by itself.

With automatic updates off, `um-upgrade-user.timer` checks for updates once a day and sends a desktop notification if there are any. "Open Updates" opens System Settings > Updates.

`um upgrade schedule` ("Apply Tonight") creates transient timers that install updates once, tonight, under the same conditions, even if automatic updates are off. "Tonight" is the configured window, or 22:00–06:00 without one. If the device is in use all night, they try again the next night. `--cancel` removes them and `--status --json` reports whether updates are scheduled.

The timers run the hidden `um upgrade run-scheduled` (root) and `um upgrade run-user` (user) commands. Setting `UM_UPGRADE_CONFIG` points `um` at another settings file and doesn't escalate, for testing.

### Packaging

Besides `/usr/bin/um` and `/usr/share/um`, packages should install:

```spec
install -Dpm 0644 polkit/org.ultramarine.um.policy %{buildroot}%{_datadir}/polkit-1/actions/org.ultramarine.um.policy
install -Dpm 0644 -t %{buildroot}%{_unitdir} systemd/system/*
install -Dpm 0644 -t %{buildroot}%{_userunitdir} systemd/user/*
install -Dpm 0644 systemd/system-preset/80-um.preset %{buildroot}%{_presetdir}/80-um.preset
install -Dpm 0644 systemd/user-preset/80-um.preset %{buildroot}%{_userpresetdir}/80-um.preset

%post
%systemd_post um-upgrade-automatic.timer
%systemd_user_post um-upgrade-user.timer

%preun
%systemd_preun um-upgrade-automatic.timer
%systemd_user_preun um-upgrade-user.timer

%postun
%systemd_postun_with_restart um-upgrade-automatic.timer
%systemd_user_postun_with_restart um-upgrade-user.timer
```

The presets enable both timers; whether updates actually install by themselves is decided by the `automatic` setting.

## Privilege escalation

Commands that need root re-execute `um` as root. From a terminal this uses `sudo` and forwards `UM_*` variables. Without a terminal (e.g. when started from the Plasma KCMs), `sudo` can't prompt, so `um` uses `pkexec` and the desktop's polkit agent shows the prompt. `pkexec` exits with 126 if the prompt is dismissed and 127 if authorization fails. It runs `um` with a clean environment, so `UM_*` overrides aren't forwarded. The `org.ultramarine.um.manage` polkit action in `polkit/org.ultramarine.um.policy` matches `/usr/bin/um` and keeps authorization for a few minutes (`auth_admin_keep`). Packages should install it to `/usr/share/polkit-1/actions/`.

## Tweaks

`um tweaks list --json` exposes the available tweaks for desktop integrations such as the Plasma Tweaks KCM. Each entry includes `id`, `description`, `type` (`toggle`, `oneshot` or `script`), `stability`, `warning` and `enabled`. Listing does not require root.

Enabled state is tracked with world-readable marker files in `/var/lib/um/tweaks/<id>.enabled` (or `$UM_STATE/tweaks`). They are written after an `enable` playbook succeeds and removed after `disable` succeeds. Tweaks enabled with an older `um` have no marker and are reported as disabled until they are enabled again.

`um tweaks enable`, `disable` and `run` all accept `--yes` for non-interactive use.

Ultramarine Tweak playbooks require the following Ansible collections as RPM packages:

- ansible-collection-ansible-posix

## Hacking
### Dev Containers

1. Install the Dev Containers extension in your IDE
   - Zed comes with Dev Containers, see [this documentation](https://zed.dev/docs/dev-containers)

   - VSCode users need to install the Dev Containers extension

   - Podman users need to install `podman-docker` from their package manager
   
2. Open your IDE and select the "Reopen in Dev Container" option
3. Use the Go toolchain as you normally would
   
### Flox
1. Get [Flox](https://flox.dev/docs/install-flox/install/)
2. Clone and enter this repo
3. Run `flox activate`
4. Use the Go toolchain as you normally would
   
### On the Host
1. Install these packages (or equivalent on another distro)  
```
golang
ansible
flatpak
flatpak-devel
rpm-devel
ansible-collection-ansible-posix
```
2. Use the Go toolchain
