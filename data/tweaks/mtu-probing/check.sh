#!/bin/sh
# only /etc/sysctl.conf is checked, not the live value. disable.yml doesn't
# reset it, and Ultramarine's own sysctl.d drop-in may set it too.
[ -r /etc/sysctl.conf ] || exit 100
grep -Eq '^[[:space:]]*net\.ipv4\.tcp_mtu_probing[[:space:]]*=[[:space:]]*1[[:space:]]*$' /etc/sysctl.conf || exit 1
