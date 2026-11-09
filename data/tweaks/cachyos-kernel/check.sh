#!/bin/sh
# only the kernel package is checked. The repo file may have come from
# `dnf copr enable` instead of enable.yml.
rpm -q --quiet kernel-cachyos || exit 1
