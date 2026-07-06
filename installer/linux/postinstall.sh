#!/bin/sh
set -e

if ! getent group viewred >/dev/null 2>&1; then
    groupadd --system viewred
fi
if ! getent passwd viewred >/dev/null 2>&1; then
    useradd --system --no-create-home --group viewred --shell /usr/sbin/nologin viewred
fi

# Copy example config if no user config exists
if [ -f /etc/viewred/config.example.yaml ] && [ ! -f /etc/viewred/config.yaml ]; then
    cp /etc/viewred/config.example.yaml /etc/viewred/config.yaml
    chown viewred:viewred /etc/viewred/config.yaml
    chmod 640 /etc/viewred/config.yaml
fi

chown -R viewred:viewred /etc/viewred

systemctl daemon-reload
systemctl enable viewred 2>/dev/null || true
