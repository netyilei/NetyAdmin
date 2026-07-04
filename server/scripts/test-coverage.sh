#!/usr/bin/env bash
# test-coverage.sh — Service 层测试覆盖率生成与门禁检查
#
# 用法：
#   cd server
#   bash scripts/test-coverage.sh
#
# 输出：
#   cover.out     — Go 覆盖率 profile（二进制格式，可被 coveralls / codecov 消费）
#   cover.html    — 可读 HTML 覆盖率报告（便于本地查看）
#
# 门禁规则：
#   Service 层（internal/service/...）覆盖率 ≥ 70%
#   未达标时脚本以非零退出码退出，CI 应据此拦截合并。
#
# 注意：
#   - 不生成 CI 配置文件（spec 9.7 要求：仅提供脚本与文档，CI 集成由各项目自行接入）
#   - -coverpkg=./... 让覆盖率统计覆盖所有包，避免跨包调用被记为零覆盖
set -euo pipefail

cd "$(dirname "$0")/.."  # 切到 server/ 目录

echo "==> Running tests with coverage..."
go test -coverprofile=cover.out -coverpkg=./... ./...

echo "==> Generating HTML report..."
go tool cover -html=cover.out -o cover.html

echo "==> Checking Service layer coverage (≥ 70%)..."
SERVICE_COV=$(go tool cover -func=cover.out | grep "^NetyAdmin/internal/service/" | \
    awk '{sum+=$3; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')

echo "    Service layer coverage: ${SERVICE_COV}%"
if (( $(echo "${SERVICE_COV%.*} < 70" | bc -l 2>/dev/null || echo 1) )); then
    echo "    [FAIL] Service layer coverage ${SERVICE_COV}% is below 70% gate"
    exit 1
fi
echo "    [PASS] Service layer coverage ${SERVICE_COV}% meets 70% gate"

echo "==> Done. Reports: cover.out, cover.html"
