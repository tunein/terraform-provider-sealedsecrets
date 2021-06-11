#! /usr/bin/env bash

set -Eeuo pipefail

hack_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
source "${hack_dir}/functions.sh"

asdf plugin add yq || true
asdf plugin add go-jsonnet https://gitlab.com/craigfurman/asdf-go-jsonnet.git || true
asdf plugin add jb https://github.com/beardix/asdf-jb.git || true
asdf plugin add jq https://github.com/focused-labs/asdf-jq.git || true
asdf plugin add golang || true
asdf plugin add terraform || true

asdf install
