#!/bin/sh
# Reports whether this tweak is enabled: exit 0 = enabled, 1 = disabled,
# 100 = cannot tell (um falls back to its own record). Runs unprivileged.
[ -f /etc/pki/tls/openssl.d/legacy-negotiation.cnf ] || exit 1
