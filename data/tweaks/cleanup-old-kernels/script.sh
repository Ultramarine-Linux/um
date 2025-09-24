#!/bin/bash
sudo dnf remove -y $(sudo dnf repoquery --installonly --latest-limit -2 -q)
