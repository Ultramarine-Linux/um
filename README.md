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

`um upgrade list --json` exposes a structured update inventory for desktop integrations such as the Plasma Updates KCM. Each entry includes `name`, `source`, `old_version`, `new_version`, and `icon`, for both RPM and Flatpak updates.

Use `um upgrade apply --yes` explicitly when scripting or integrating the operation. The `--yes` flag is also available on the top-level `um upgrade` command.

## Tweaks

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
