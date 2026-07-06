#!/bin/sh
set -e

systemctl stop viewred 2>/dev/null || true
systemctl disable viewred 2>/dev/null || true
