#!/bin/bash
set -euo pipefail

if [ -e /run/ostree-booted ]; then
	echo "this tweak isn't supported on atomic systems, old deployments are cleaned up by bootc/rpm-ostree" >&2
	exit 1
fi

# keep the two newest kernels, and never the one we're running from
mapfile -t old < <(dnf5 repoquery --installonly --latest-limit -2 -q | grep -vF ":$(uname -r)" | grep . || true)

if [ ${#old[@]} -eq 0 ]; then
	echo "no old kernels to remove"
	exit 0
fi

dnf5 remove -y "${old[@]}"
