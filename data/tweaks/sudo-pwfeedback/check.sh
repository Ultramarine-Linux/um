#!/bin/sh
# /etc/sudoers.d is usually not traversable by regular users
[ -x /etc/sudoers.d ] || exit 100
[ -f /etc/sudoers.d/pwfeedback ] || exit 1
