#!/bin/sh
# Reports whether this tweak is enabled: exit 0 = enabled, 1 = disabled,
# 100 = cannot tell (um falls back to its own record). Runs unprivileged.
[ -e /nix/receipt.json ] || exit 1
