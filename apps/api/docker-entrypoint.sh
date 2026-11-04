#!/bin/sh
# The data volume is created as root. This script gives it to the panel user,
# then replaces itself with the API. It is not a general-purpose shell.
set -eu
mkdir -p /data
chown altas:altas /data
exec su-exec altas /usr/local/bin/altas-api
