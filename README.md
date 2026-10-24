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

## Tweaks

Ultramarine Tweak playbooks require the following Ansible collections as RPM packages:

- ansible-collection-ansible-posix

Each tweak is a directory under `/usr/share/um/tweaks/<id>` with a `metadata.yml`
and, depending on its `type`, `enable.yml`/`disable.yml` (toggle), `enable.yml`
(oneshot) or `script.sh` (script).

Toggle and oneshot tweaks may ship an executable `check.sh` that reports whether
the tweak is enabled. It runs unprivileged (so `um tweaks list` and the Tweaks KCM
never need a password) and exits 0 if enabled, 1 if disabled, or 100 if it
can't tell (other codes are reported as errors). It's run with `/bin/sh`.
Without a `check.sh`, or when it exits 100, um uses the marker it writes to
`/var/lib/um/tweaks/<id>.enabled` when the tweak is enabled through um.

`um tweaks list --json` prints `{"version": 1, "tweaks": [...]}`. Entries have
`state_error` set when their state couldn't be determined.

Packaging: install `polkit/org.ultramarine.um.policy` to
`/usr/share/polkit-1/actions/`, install `um` as a regular file at `/usr/bin/um`
(pkexec matches the resolved path), and ship `/var/lib/um` and
`/var/lib/um/tweaks` as root-owned `0755` directories.

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
