#!/bin/sh
[ -r /proc/sys/net/ipv4/tcp_mtu_probing ] || exit 100
[ "$(cat /proc/sys/net/ipv4/tcp_mtu_probing)" != 0 ] || exit 1
