set -ex

cd $(git rev-parse --show-toplevel)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build  \
    -ldflags="-s -w" \
    -o ./tools/logs_compactor/logcompactor \
    ./tools/logs_compactor

DOCKER_BUILDKIT=1 docker build -f build/Dockerfiles/log-compactor/Dockerfile.fast -t logcompactor:latest .

set +x