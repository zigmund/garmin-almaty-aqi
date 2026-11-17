#!/bin/sh
# Run a make command for the watch app inside the Connect IQ SDK image (same as CI).
# Usage: scripts/ciq-docker.sh KEY_DIR make-args...   (KEY_DIR holds developer_key.der)
# The image (Alpine) has the SDK and every device; bash/make/zip/bsdtar are added per run.
set -eu
# Pinned by digest (multi-arch index of tag 9.3.0, resolved with `docker buildx imagetools inspect`):
# the release job mounts the real signing key into this image. Bump tag and digest together.
IMAGE=ghcr.io/blackshadev/garmin-connectiq-tools:9.3.0@sha256:5188a3bf7cf6511cb82c025666a4025edd0ec12218425a29e18d50c7594ca865
key_dir=$(cd "$1" && pwd); shift
root=$(cd "$(dirname "$0")/.." && pwd)
exec docker run --rm --platform linux/amd64 \
    -v "$root/watch":/w -v "$key_dir":/k:ro -w /w \
    --entrypoint sh "$IMAGE" -c 'apk add -q bash make zip libarchive-tools >/dev/null && exec make "$@" SDK=/connectiq DEVICES_DIR=/root/.Garmin/ConnectIQ/Devices KEY=/k/developer_key.der' make "$@"
