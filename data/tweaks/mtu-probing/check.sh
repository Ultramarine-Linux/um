#!/bin/sh
# Reports whether this tweak is enabled: exit 0 = enabled, 1 = disabled,
# 100 = cannot tell (um falls back to its own record). Runs unprivileged.
# enable.yml/disable.yml manage the setting in /etc/sysctl.conf. The live value
# isn't checked: disable.yml doesn't reset it, and Ultramarine ships its own
# sysctl.d drop-in that may set it too.
[ -r /etc/sysctl.conf ] || exit 100
grep -Eq '^[[:space:]]*net\.ipv4\.tcp_mtu_probing[[:space:]]*=[[:space:]]*1[[:space:]]*$' /etc/sysctl.conf || exit 1
