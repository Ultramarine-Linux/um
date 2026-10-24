#!/bin/sh
# Reports whether this tweak is enabled: exit 0 = enabled, 1 = disabled,
# 100 = cannot tell (um falls back to its own record). Runs unprivileged.
# /etc/sudoers.d is usually not readable by regular users.
[ -x /etc/sudoers.d ] || exit 100
[ -f /etc/sudoers.d/pwfeedback ] || exit 1
