# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

`gocp` 是一个基于 Go 的图像压缩库，使用 OpenCV (gocv) 实现。主要功能包括：
- 图像有损压缩（支持 JPG/WebP 格式）
- 图像格式转换
- 智能尺寸调整（仅缩小，不放大）
- HSV 色彩空间绿色背景移除 (`Optimize` 函数)
- 支持文件路径和字节流两种输入方式

## 依赖

- **Go 1.22.6+**
- **gocv.io/x/gocv** (v0.42.0) - OpenCV Go 绑定，需要系统安装 OpenCV
- **github.com/panjf2000/ants/v2** (v2.0.0) - 协程池，用于批量处理

**注意**: 系统必须预装 OpenCV 库，否则 gocv 无法正常工作。

## 构建与测试

### Go 命令

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run Test_Work ./...
go test -v -run TestMemoryLeakDetection ./...

# 运行特定包的测试
go test -v ./...

# 测试覆盖率
go test -cover ./...

# 基准测试
go test -bench=. -benchmem ./...

# 构建模块
go build ./...

# 检查依赖
go mod tidy
go mod verify

# 构建 CLI 工具
go build -o gocp ./cmd/gocp
```

### Docker 构建命令

```bash
# 本地构建（当前架构）
make build-local

# 多架构构建（amd64, arm64）
make build

# 测试 Docker 镜像
make test

# 清理镜像
make clean

# 进入容器 shell
make shell

# 检查多架构 manifest
make inspect

# Lint Dockerfile（需要 hadolint）
make lint
```

## 代码架构

### 包结构

项目采用单包设计 (`package cp`)，所有源代码文件位于根目录：

### 核心文件说明

#### **work.go** - 主要压缩逻辑

**公开函数**:
- `Compress(imgPath string, params *ImageCompressor) ([]byte, error)` - 从文件路径压缩图像
- `CompressByBytes(src []byte, params *ImageCompressor) ([]byte, error)` - 从字节数组压缩图像
- `Optimize(src gocv.Mat) (gocv.Mat, error)` - HSV 色彩空间绿色背景移除，替换为白色背景

**私有函数**:
- `resize(img gocv.Mat, resized *gocv.Mat, width, height int) error` - 图像缩放，使用 `InterpolationArea` 插值算法
- `encodeImage(img gocv.Mat, quality int, format string) ([]byte, error)` - 图像编码，先应用高斯模糊再压缩
- `fileSize(path string) (int64, error)` - 获取文件大小

**图像处理流程**:
```
输入图像 → 可选Resize → 高斯模糊(3x3) → 参数化编码 → 输出字节流
```

#### **options.go** - 函数式选项模式

- `ImageCompressor` 结构体：包含 Quality, Format, Resize, Height, Width 配置
- `Option` 类型：`func(*ImageCompressor)` 函数类型
- `NewImageCompressor(opts ...Option) *ImageCompressor` - 构造函数
- `WithQuality(quality int) Option` - 设置压缩质量 (0-100)
- `WithFormat(format string) Option` - 设置输出格式 ("jpg" 或 "webp")
- `WithResize(resize bool, width, height int) Option` - 设置尺寸调整

#### **chains.go** - 责任链模式（预留，当前已注释）

包含责任链模式的预留实现，用于构建可扩展的图像处理管道。当前未启用，保留用于未来扩展。

#### **work_test.go** - 测试示例

- `Test_Work` - 批量图像处理测试，使用 `ants` 协程池并行处理
- `walkImages(root string)` - 递归遍历目录获取所有图像文件
- `isImageExt(name string)` - 检查文件扩展名是否为支持的图像格式

### gocv.Mat 资源管理（关键）

gocv.Mat 是 OpenCV 矩阵对象的 Go 包装，**需要手动管理内存**：

```go
// 正确：使用 defer 确保释放
img := gocv.IMRead(path, gocv.IMReadColor)
defer img.Close()

// 克隆需要再次管理
resized := gocv.NewMat()
defer resized.Close()

// Clone 返回新的 Mat，需要接管所有权
img.Close()  // 释放原 Mat
img = resized.Clone()  // 获得新 Mat 的所有权
```

**资源管理规则**:
1. 所有 `gocv.NewMat*()` 创建的对象都需要 `Close()`
2. `IMRead`, `IMDecode` 返回的对象需要 `Close()`
3. 使用 `defer` 确保资源释放，即使在错误发生时
4. `Clone()` 会创建新的 Mat，新对象也需要 `Close()`
5. **返回 Mat 的函数（如 `Optimize`），调用者必须负责关闭返回值**

## API 使用示例

### 基本用法

```go
import cp "github.com/cytzrs/gocp"

// 方式1: 直接构造参数
buf, err := cp.Compress(imgPath, &cp.ImageCompressor{
    Format:  "jpg",
    Quality: 75,
    Resize:  true,
    Height:  4096,
    Width:   3072,
})

if err != nil {
    log.Fatal(err)
}
```

### 选项模式（推荐）

```go
// 方式2: 使用选项模式（更灵活、可扩展）
params := cp.NewImageCompressor(
    cp.WithQuality(75),
    cp.WithFormat("jpg"),
    cp.WithResize(true, 3072, 4096),
)
buf, err := cp.Compress(imgPath, params)
```

### 从字节流压缩

```go
// 读取已有字节流（如从 HTTP 请求获取）
srcBytes, _ := os.ReadFile("input.png")
buf, err := cp.CompressByBytes(srcBytes, params)
```

### 批量处理（使用协程池）

```go
import "github.com/panjf2000/ants/v2"

pool, _ := ants.NewPool(5, // 并发数
    ants.WithExpiryDuration(10*time.Minute),
    ants.WithPreAlloc(true),
)

for _, imgPath := range imageFiles {
    path := imgPath // 闭包捕获
    pool.Submit(func() {
        buf, err := cp.Compress(path, params)
        // 处理结果...
    })
}
```

## CLI 工具使用

项目包含一个命令行工具 `cmd/gocp/main.go`，可直接运行图像压缩：

### 构建 CLI

```bash
go build -o gocp ./cmd/gocp
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-quality` | 75 | 压缩质量 (0-100) |
| `-format` | jpg | 输出格式 (jpg, webp) |
| `-resize` | false | 是否调整尺寸 |
| `-width` | 1920 | 目标宽度 |
| `-height` | 1080 | 目标高度 |
| `-version` | - | 显示版本信息 |
| `-help` | - | 显示帮助 |

### 使用示例

```bash
# 基本压缩
./gocp input.jpg output.jpg

# 高质量 WebP 输出
./gocp -quality 85 -format webp input.png output.webp

# 调整尺寸并压缩
./gocp -resize -width 3072 -height 4096 input.jpg output.jpg

# 查看版本
./gocp -version
```

## 压缩参数建议

### Quality（压缩质量）

| 格式  | 范围   | 推荐值    | 说明                     |
|-------|--------|-----------|--------------------------|
| JPG   | 0-100  | 75-85     | 平衡质量和文件大小       |
| WebP  | 0-100  | 80-90     | 相同质量下文件更小       |

### Resize（尺寸调整）

- 算法：`gocv.InterpolationArea` - 适合缩小图像
- 行为：**仅缩小，不放大**（使用 `min()` 确保不超过原图尺寸）
- 常用尺寸：
  - 4K: `4096 x 3072`
  - 1080p: `1920 x 1080`
  - 缩略图: `300 x 300`

### 格式选择

| 格式  | 优点                          | 缺点              | 适用场景           |
|-------|-------------------------------|-------------------|--------------------|
| JPG   | 兼容性好，压缩率高            | 无透明通道        | 照片、复杂图像     |
| WebP  | 压缩率更高，支持有损/无损     | 兼容性稍差        | Web 应用、现代环境 |

## 图像处理管道

### 完整处理流程

```
1. 图像解码 (IMRead/IMDecode)
   ↓
2. 可选: 尺寸调整 (Resize with InterpolationArea)
   ↓
3. 高斯模糊 (GaussianBlur 3x3, sigma=1.0)
   - 目的: 减少噪点，提高压缩效率
   ↓
4. 参数化编码 (IMEncodeWithParams)
   - JPG: IMWriteJpegQuality, IMWriteJpegOptimize, IMWriteJpegChromaQuality
   - WebP: IMWriteWebpQuality
   ↓
5. 输出字节流
```

### Optimize 函数（绿色背景移除）

**⚠️ 重要：返回的 Mat 必须由调用者手动关闭！**

```go
// ✅ 正确的使用方式
src := gocv.IMRead("input.jpg", gocv.IMReadColor)
defer src.Close()

optimized, err := cp.Optimize(src)
if err != nil {
    log.Fatal(err)
}
defer optimized.Close()  // 必须关闭！否则会内存泄漏

// 使用 optimized...
gocv.IMWrite("output.jpg", optimized)
```

**错误示例（会内存泄漏）**：
```go
// ❌ 错误：忘记关闭返回的 Mat
optimized, _ := cp.Optimize(src)
// 没有 defer optimized.Close()
// 函数返回后，底层 C 内存泄漏！
```

**并发场景**：
```go
// ✅ 并发使用时，每个协程都要负责关闭
var wg sync.WaitGroup
for _, img := range images {
    wg.Add(1)
    go func(m gocv.Mat) {
        defer wg.Done()
        defer m.Close()  // 关闭输入

        result, err := cp.Optimize(m)
        if err != nil {
            return
        }
        defer result.Close()  // 关闭输出

        // 处理 result...
    }(img)
}
```

**功能说明**：
- HSV 绿色范围: H(35-77), S(43-255), V(46-255)
- 将绿色背景替换为白色 (255, 255, 255)
- 适用于证件照、绿幕图像等场景

## 错误处理

所有公开函数都返回 `error`，必须检查：

```go
buf, err := cp.Compress(path, params)
if err != nil {
    // 常见错误:
    // - "failed to read image" - 图像文件损坏或格式不支持
    // - "failed to resize image" - 尺寸参数无效
    // - "failed to encode image" - 编码参数错误
    log.Printf("压缩失败: %v", err)
    return
}

if buf == nil {
    log.Println("输出缓冲区为空")
    return
}
```

## 支持的图像格式

### 输入格式
- `.jpg`, `.jpeg`
- `.png`
- `.bmp`
- `.tiff`
- `.webp`

### 输出格式
- `.jpg` / `.jpeg` (默认)
- `.webp`

## 性能优化建议

1. **批量处理**: 使用 `ants` 协程池，并发数建议 5-10
2. **内存管理**: 及时 `Close()` Mat 对象，避免内存泄漏
3. **质量控制**: 根据场景选择合适质量，避免过度压缩
4. **尺寸策略**: 优先缩小尺寸以获得更好的压缩效果

## 开发注意事项

1. **不要修改 gocv.Mat 的生命周期**：确保 Close() 在正确的时机调用
2. **克隆会创建新对象**：Clone() 返回的 Mat 需要独立管理
3. **错误处理要完整**：检查所有返回的错误，包括空缓冲区
4. **参数验证**：Quality 范围 0-100，Width/Height 应为正整数
5. **格式转换**：从透明格式 (PNG) 转换到 JPG 时，透明通道会被合并

## 已修复的内存问题

### v0.1.0 修复：`encodeImage` 的 use-after-free 漏洞（严重）

**问题根源**：gocv 的 `NativeByteBuffer.GetBytes()` 返回的是 **C 内存的视图（view）**，而非 Go 堆上的副本。

```go
// gocv v0.42.0 的实际实现
func (buffer *NativeByteBuffer) GetBytes() []byte {
    var result []byte
    sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&result))
    sliceHeader.Data = uintptr(buffer.dataPointer())  // ⚠️ 直接指向 C 内存！
    return result
}
```

**v0.0.8 的错误修复**：
```go
// ❌ v0.0.8 的修复（仍然有 bug！）
buf, err := gocv.IMEncodeWithParams(gocv.FileExt(ext), dst, params)
if err != nil {
    return nil, err
}
defer buf.Close()  // 函数返回时释放 C 内存
return buf.GetBytes(), nil  // ⚠️ 返回指向已释放内存的切片！
```

**执行顺序**：
1. `buf.GetBytes()` 返回指向 C 内存的 `[]byte`
2. `defer buf.Close()` 执行，释放 C++ std::vector 内存
3. 函数返回 `[]byte`，但它指向已释放的内存
4. 调用者访问时触发 **use-after-free**（未定义行为）

**影响**：
- 🔴 **严重安全漏洞**：返回悬空指针，访问已释放的内存
- 🔴 **未定义行为**：可能崩溃、数据损坏、或被攻击者利用
- 🔴 **并发场景高危**：多协程访问时问题更明显
- 🔴 **难以调试**：Release 模式下可能"正常工作"，但任何时刻都可能出问题

**v0.1.0 正确修复**：
```go
// ✅ v0.1.0 的正确修复
buf, err := gocv.IMEncodeWithParams(gocv.FileExt(ext), dst, params)
if err != nil {
    return nil, err
}

// ⚠️ 关键：必须在 Close() 之前复制数据到 Go 堆
cBytes := buf.GetBytes()  // 获取 C 内存视图
result := make([]byte, len(cBytes))
copy(result, cBytes)  // 复制到 Go 堆

buf.Close()  // 现在可以安全释放 C 内存

return result, nil  // 返回 Go 堆上的独立副本
```

**修复验证**：
- 返回的 `[]byte` 是 Go 堆上的独立副本
- 不依赖 C 生命周期
- GC 可以正常管理
- 并发安全

### v0.0.8 修复：`Optimize` 返回已关闭的 Mat

详见上文 `Optimize` 函数说明。

## 测试数据

`Test_Work` 函数需要准备测试图像目录：
- 默认读取路径：`../destination/`
- 输出路径：`./out/`

运行测试前请确保存在有效的测试图像。

## CI/CD

### GitHub Actions

项目使用 GitHub Actions 自动构建多架构 Docker 镜像：

- **触发条件**：推送到 `main` 分支、创建 tag (`v*`)、或手动触发
- **支持架构**：`linux/amd64`, `linux/arm64`
- **镜像仓库**：`docker.io/cytzrs/gocp`
- **特性**：
  - 使用 Buildx 多架构构建
  - 支持 SBOM 和 Provenance
  - GitHub Actions 缓存加速构建

### 版本发布流程

```bash
# 1. 确保在 main 分支且代码已合并
git checkout main
git pull origin main

# 2. 更新版本号（如果有 version.go 或类似文件）
# 3. 创建 git tag 触发 CI/CD 构建
git tag v1.0.1
git push origin v1.0.1

# 4. GitHub Actions 会自动构建并推送多架构镜像到 Docker Hub
# 5. 验证镜像发布
docker pull cytzrs/gocp:v1.0.1
docker buildx imagetools inspect cytzrs/gocp:v1.0.1
```

### Docker 镜像使用

```bash
# 拉取镜像
docker pull cytzrs/gocp:latest

# 运行容器压缩图像
docker run --rm -v $(pwd):/data cytzrs/gocp:latest \
    -quality 85 -resize -width 3072 -height 4096 \
    /data/input.jpg /data/output.jpg
```

## 测试文件说明

### work_test.go

- `Test_Work` - 批量图像处理测试，使用 `ants` 协程池并行处理
- `walkImages(root string)` - 递归遍历目录获取所有图像文件
- `isImageExt(name string)` - 检查文件扩展名是否为支持的图像格式

### bytevector_test.go

用于验证 `gocv.ByteVector` 的内存管理行为：

- `TestByteVectorGetBytesBehavior` - 验证 `GetBytes()` 返回的是数据的副本（安全）而非视图
- `TestDeferAndGetBytesOrder` - 验证 `defer Close()` 后返回 `GetBytes()` 的安全性
- `BenchmarkDeferAndGetBytes` - 性能基准测试

这些测试确认了 `encodeImage` 函数中 `defer buf.Close()` 的正确性。

### memory_leak_test.go

用于验证内存泄漏问题的回归测试：

- `TestMemoryLeakDetection` - 循环调用 `CompressByBytes` 1000 次，监控 Stack 和 Heap 内存增长
- `TestMemoryLeakDetectionParallel` - 使用协程池并发测试 1000 次压缩操作

**运行内存泄漏测试**：
```bash
# 单独运行内存泄漏测试
go test -v -run TestMemoryLeakDetection ./...
go test -v -run TestMemoryLeakDetectionParallel ./...

# 运行所有测试
go test -v ./...
```

**测试说明**：
- 测试会循环压缩 1000 次，每 100 次 GC 后检查内存增长
- Stack 增长超过 10MB 会触发警告
- 最终 Stack 增长超过 20MB 会判定为内存泄漏
- 并发测试使用 16 线程池，模拟真实生产场景
