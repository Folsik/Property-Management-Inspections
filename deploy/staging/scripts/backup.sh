#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
dest=${BACKUP_DIR:-/opt/inspectly/backups}
mkdir -p "$dest"
file="$dest/staging-$(date -u +%Y%m%dT%H%M%SZ).dump"
tmp=$(mktemp "$dest/.backup.XXXXXX")
trap 'rm -f "$tmp"' EXIT
sudo docker compose exec -T postgres pg_dump -U inspections -d inspections -Fc > "$tmp"
test -s "$tmp"
sudo docker compose exec -T postgres pg_restore -l < "$tmp" >/dev/null
mv "$tmp" "$file"
echo "$file"
