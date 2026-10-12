#!/usr/bin/env bash
set -euo pipefail

# cd to project root (where this script's parent dir resides)
cd "$(cd "$(dirname "$0")" && pwd)/.."

# ============================================================
# zPush Docker build & push script
# 用法: ./build.sh [版本标签]
# 示例: ./build.sh v1.0.0
#       ./build.sh          # 不传则用 git short commit 作为标签
# ============================================================

# ---- 配置 ----
IMAGE_NAME="zhangz1o/zpush"
PROXY="${BUILD_PROXY:-http://127.0.0.1:7890}"   # 代理地址，可通过环境变量覆盖
REGISTRY_MIRROR="${REGISTRY_MIRROR:-}"            # Docker registry mirror（可选）

# ---- 确定版本标签 ----
if [ $# -ge 1 ]; then
    TAG="$1"
else
    TAG="dev-$(git rev-parse --short HEAD)"
fi

FULL_IMAGE="${IMAGE_NAME}:${TAG}"
LATEST_IMAGE="${IMAGE_NAME}:latest"

echo "============================================"
echo " Image:    ${FULL_IMAGE}"
echo " Proxy:    ${PROXY}"
echo "============================================"

# ---- 拉取最新代码（如果是 CI 环境可跳过） ----
if [ "${SKIP_PULL:-}" != "1" ]; then
    echo ">>> git pull ..."
    git pull
fi

# ---- Docker build（注入代理环境变量） ----
echo ">>> docker build ..."

docker build \
    --build-arg HTTP_PROXY="${PROXY}" \
    --build-arg HTTPS_PROXY="${PROXY}" \
    --build-arg GOPROXY="https://goproxy.cn,https://proxy.golang.org,direct" \
    -t "${FULL_IMAGE}" \
    -t "${LATEST_IMAGE}" \
    -f Dockerfile \
    .

echo ">>> build 成功 ✓"

# ---- 推送到 Docker Hub ----
echo ">>> docker push ${FULL_IMAGE}"
docker push "${FULL_IMAGE}"

echo ">>> docker push ${LATEST_IMAGE}"
docker push "${LATEST_IMAGE}"

echo "============================================"
echo " 推送完成 ✓"
echo "   ${FULL_IMAGE}"
echo "   ${LATEST_IMAGE}"
echo "============================================"
