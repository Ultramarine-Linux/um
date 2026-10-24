#!/bin/sh
# Reports whether this tweak is enabled: exit 0 = enabled, 1 = disabled,
# 100 = cannot tell (um falls back to its own record). Runs unprivileged.
# The kernel package is what enable.yml installs and disable.yml removes; the
# repo file isn't checked since it may have been added with `dnf copr enable`.
rpm -q --quiet kernel-cachyos || exit 1
