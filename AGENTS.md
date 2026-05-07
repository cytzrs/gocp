# AGENTS.md

> gocp — 基于 Go + OpenCV (gocv) 的图像压缩库，单包设计 (`package cp`)。

## 核心约束

- **系统依赖**：必须预装 OpenCV，否则 `gocv` 编译/运行失败。Docker 构建走 `gocv/opencv:latest` 基础镜像。
- **CGo**：`umat_impl.go` 包含 CGo 代码（OpenCL GPU 加速），构建时需要 `CGO_ENABLED=1`。
- **内存管理（致命）**：`gocv.Mat` 底层是 C 对象，**必须手动 Close()**。`Optimize()` 返回的 Mat 由调用者负责关闭，否则内存泄漏。
- **use-after-free 陷阱**：`gocv.ByteVector.GetBytes()` 返回 C 内存视图而非 Go 副本。`encodeImage` 中必须先 `copy()` 到 Go 堆再 `Close()`，顺序不可颠倒。

## 命令速查

```bash
# 测试
go test -v ./...                          # 全部测试
go test -v -run TestMemoryLeakDetection   # 内存泄漏回归测试
go test -bench=. -benchmem ./...          # 基准测试

# 构建
go build -o gocp ./cmd/gocp               # CLI 工具
go mod tidy && go mod verify              # 依赖校验

# Docker
make build-local                          # 本地镜像
make build                                # 多架构镜像 (amd64 + arm64)
make test                                 # 验证镜像
```

## 项目结构

```
work.go         — Compress/CompressByBytes/Optimize 公开函数
options.go      — ImageCompressor + 函数式选项模式
umat_impl.go    — CGo UMat/OpenCL GPU 加速（fallback 到 CPU）
chains.go       — 责任链模式（已注释，预留）
cmd/gocp/       — CLI 入口
```

## 关键架构笔记

- **单包设计**：所有源码在根目录，`package cp`，无子包。
- **图像处理流程**：解码 → 可选 Resize（仅缩小，`InterpolationArea`）→ 高斯模糊 3x3 → 编码输出。
- **编码参数**：JPG 走 `IMWriteJpegQuality` + `IMWriteJpegOptimize` + `IMWriteJpegChromaQuality`；WebP 走 `IMWriteWebpQuality`。
- **GPU 入口**：`CompressWithGPU()`/`CompressByBytesWithGPU()`，自动检测 OpenCL 可用性，不可用时 fallback 到 CPU 版本。
- **测试图片**：`work_test.go` 中 `Test_Work` 需要 `../destination/` 目录存在有效图片；`memory_leak_test.go` 自动生成测试图片。

## CI/CD

- GitHub Actions (`docker-build.yml`)：push main 或 tag `v*` 触发多架构 Docker 镜像构建，推送到 `docker.io/cytzrs/gocp`。
- 发版：`git tag v1.x.x && git push origin v1.x.x`。

## 详细文档

- `CLAUDE.md` — 完整 API 文档、内存管理规则、历史 bug 修复记录。
- `aidocs/` — 内存泄漏修复说明和快速参考。
