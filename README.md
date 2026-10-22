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

`um upgrade` upgrades the system's RPM and Flatpak packages in one operation, similar to topgrade.

`um upgrade list --json` exposes a structured update inventory for desktop integrations such as the Plasma Updates KCM. Each entry includes `id`, `name`, `source`, `old_version`, `new_version`, and `icon`, for both RPM and Flatpak (system and user installation) updates. `id` is unique: `name.arch` for RPMs, `app.id//branch` for Flatpaks (suffixed with `@user` for the user installation). Sources that fail to be queried are reported in `errors` while updates from the remaining sources are still listed; the command only exits non-zero when no source could be queried.

Without `--yes`, `dnf` and `flatpak` prompt for confirmation as usual. User-installation Flatpaks are updated before escalating to root. Use `um upgrade apply --yes` explicitly when scripting or integrating the operation. The `--yes` flag is also available on the top-level `um upgrade` command.

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
