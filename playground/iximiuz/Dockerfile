# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS gobuild
ENV GOPROXY=off GOFLAGS=-mod=mod
WORKDIR /src/swap-rithms
COPY swap-rithms/go.mod ./
COPY swap-rithms/*.go ./
COPY swap-rithms/workers ./workers
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/swap-rithms .

FROM node:24-bookworm-slim AS nodebuild

FROM ghcr.io/iximiuz/labs/rootfs:ubuntu-24-04

USER root
ENV HOME=/root

COPY swap-rithms /opt/swap-rithms
COPY playground/iximiuz/image/swap-rithms.service /etc/systemd/system/swap-rithms.service
COPY --from=gobuild /out/swap-rithms /opt/swap-rithms/swap-rithms
COPY --from=nodebuild /usr/local/bin/node /usr/local/bin/node

RUN <<'EOF'
set -eu

apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  bash \
  ca-certificates \
  curl \
  jq \
  libstdc++6 \
  procps \
  python3
rm -rf /var/lib/apt/lists/*

chown -R root:root /opt/swap-rithms
chmod 755 /opt/swap-rithms/swap-rithms

cat > /usr/local/bin/swap-rithms <<'EOF_WRAPPER'
#!/usr/bin/env bash
exec /opt/swap-rithms/swap-rithms "$@"
EOF_WRAPPER
chmod 755 /usr/local/bin/swap-rithms

if id laborant >/dev/null 2>&1; then
  ln -sfn /opt/swap-rithms /home/laborant/swap-rithms
  chown -h laborant:laborant /home/laborant/swap-rithms
  grep -qxF 'cd ~/swap-rithms' /home/laborant/.bashrc || echo 'cd ~/swap-rithms' >> /home/laborant/.bashrc
fi

for cmd in bash curl jq node python3 swap-rithms; do
  command -v "${cmd}" >/dev/null 2>&1 || {
    echo "missing expected tool: ${cmd}" >&2
    exit 1
  }
done

node --experimental-strip-types -e 'type T = { id: number }; const x: T = { id: 1 }; if (x.id !== 1) process.exit(1)'
python3 -c 'import json; print(json.dumps({"ok": True}))' >/dev/null

ln -sf /etc/systemd/system/swap-rithms.service \
  /etc/systemd/system/multi-user.target.wants/swap-rithms.service
EOF

USER laborant
ENV HOME=/home/laborant
WORKDIR /home/laborant
