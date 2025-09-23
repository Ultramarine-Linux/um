#!/bin/bash
sudo dnf remove -y $(dnf repoquery --installonly --latest-limit -2 -q)
