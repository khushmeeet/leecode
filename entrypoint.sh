#!/bin/sh
set -e

# On a brand-new machine (first deploy, or after a Fly host/volume loss) the
# local DB file won't exist. Pull the latest snapshot from the replica before
# starting the app. -if-replica-exists makes the very first deploy (empty
# bucket) a clean no-op instead of a hard failure.
if [ ! -f "$DB_PATH" ]; then
  echo "litestream: $DB_PATH not found, restoring from replica (if any)"
  litestream restore -if-replica-exists "$DB_PATH"
fi

# Replicate continuously and run the app as a child process. Litestream forwards
# signals and performs a final sync when the app exits (e.g. on Fly auto-stop).
exec litestream replicate -exec "/leecode"
