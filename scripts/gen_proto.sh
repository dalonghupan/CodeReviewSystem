#!/usr/bin/env bash
# 重新生成 protobuf 代码（P7）
# 依赖：buf、protoc-gen-go、protoc-gen-go-grpc、protoc-gen-go-http、protoc-gen-validate
# 安装：go install <plugin>@latest（见 TASKS.md）
set -euo pipefail

cd "$(dirname "$0")/.."
export PATH="$PATH:$(go env GOPATH)/bin"

# 仅生成 proto/crsystem/v1 下的业务文件（third_party 仅作为依赖参与编译）
buf generate --path proto/crsystem/v1

echo "生成完成 → api/crsystem/v1/"
