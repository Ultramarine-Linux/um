#!/bin/sh
[ -x /etc/sudoers.d ] || exit 100
[ -f /etc/sudoers.d/pwfeedback ] || exit 1
