#!/bin/sh
# the live value is checked since Ultramarine's own sysctl.d drop-in may also set it
[ -r /proc/sys/net/ipv4/tcp_mtu_probing ] || exit 100
[ "$(cat /proc/sys/net/ipv4/tcp_mtu_probing)" != 0 ] || exit 1
