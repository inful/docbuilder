#!/bin/sh
# Smoke test for the Docker image: the binary runs, the daemon starts and
# answers, and the toolchain in the image works.
#
# The image is what an operator runs, and nothing else ran it: the CI
# built the image but never started it. A broken image (a work directory
# the daemon cannot write, a home directory the Go module cache cannot
# use) passed the build and failed at the first start.
#
# The test needs no network: the repository in the configuration is
# never cloned, because the daemon clones on the first build, not when
# it starts.
#
# Usage: scripts/docker-smoke.sh IMAGE
set -eu

IMAGE="${1:?usage: docker-smoke.sh IMAGE}"

WORK_DIR="$(mktemp -d)"
NAME="docbuilder-smoke-$$"
DOCS_PORT="${SMOKE_DOCS_PORT:-18080}"

cleanup() {
  docker rm -f "${NAME}" >/dev/null 2>&1 || true
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

cat > "${WORK_DIR}/config.yaml" <<YAML
version: "2.0"
repositories:
  - name: smoke
    url: https://example.com/smoke.git
    branch: main
    paths: [docs]
daemon:
  http:
    docs_port: ${DOCS_PORT}
    webhook_port: $((DOCS_PORT + 1))
    admin_port: $((DOCS_PORT + 2))
  storage:
    repo_cache_dir: "./repositories"
output:
  directory: "./site"
YAML

echo "== the binary runs =="
docker run --rm "${IMAGE}" --version

echo "== the daemon starts and answers =="
docker run --rm -d --name "${NAME}" \
  -v "${WORK_DIR}/config.yaml:/config/config.yaml:ro" \
  -p "127.0.0.1:${DOCS_PORT}:${DOCS_PORT}" \
  "${IMAGE}" daemon --config /config/config.yaml >/dev/null

# The daemon exits when it cannot write its state, so a container that is
# still up after a moment has a writable work directory.
sleep 5
if [ "$(docker inspect -f '{{.State.Running}}' "${NAME}")" != "true" ]; then
  echo "the daemon stopped:" >&2
  docker logs "${NAME}" >&2 || true
  exit 1
fi

status="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${DOCS_PORT}/" || true)"
if [ "${status}" = "000" ] || [ -z "${status}" ]; then
  echo "the docs port did not answer:" >&2
  docker logs "${NAME}" >&2 || true
  exit 1
fi
echo "the docs port answered with ${status}"

echo "== the toolchain in the image works =="
# Hugo renders the site and Hugo Modules are Go modules, so the image
# carries a Go toolchain. The module cache lives in the home directory
# of the user that runs the daemon: if that is not writable, every
# build fails.
docker run --rm --entrypoint /bin/bash "${IMAGE}" -c '
  set -eu
  go version
  hugo version
  mkdir -p "$(go env GOPATH)/pkg/mod"
  echo "the module cache is writable"
'

echo "== the image is fine =="
