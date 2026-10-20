#!/bin/sh
set -eu

pm2 start sandbox -- -mount-conf /root/.hydro/mount.yaml
exec pm2-runtime start hydrojudge
