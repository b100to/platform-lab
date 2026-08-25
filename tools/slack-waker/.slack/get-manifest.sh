#!/usr/bin/env sh
# The CLI appends flags such as --source to hook commands, so the hook has to
# be something that ignores its arguments rather than `cat manifest.json`.
cd "$(dirname "$0")/.." || exit 1
cat manifest.json
