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

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run Test_Work ./...

# 构建模块
go build ./...

# 测试覆盖率
go test -cover ./...

# 检查依赖
go mod tidy
go mod verify
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

## 已修复的内存泄漏问题

### v0.0.8 修复：`encodeImage` 的 ByteVector 泄漏

**问题**：`gocv.IMEncodeWithParams` 返回的 `ByteVector` 没有被 `Close()`

```go
// ❌ 修复前（内存泄漏）
buf, err := gocv.IMEncodeWithParams(gocv.FileExt(ext), dst, params)
if err != nil {
    return nil, err
}
return buf.GetBytes(), nil  // buf 没有关闭！
```

**影响**：
- 每次调用 `Compress` / `CompressByBytes` 泄漏 C++ std::vector 内存
- 并发调用时内存快速增长
- 长时间运行可能导致 OOM

**修复**：
```go
// ✅ 修复后
buf, err := gocv.IMEncodeWithParams(gocv.FileExt(ext), dst, params)
if err != nil {
    return nil, err
}
defer buf.Close()  // 释放 C 内存
return buf.GetBytes(), nil
```

### v0.0.8 修复：`Optimize` 返回已关闭的 Mat

详见上文 `Optimize` 函数说明。

## 测试数据

`Test_Work` 函数需要准备测试图像目录：
- 默认读取路径：`../destination/`
- 输出路径：`./out/`

运行测试前请确保存在有效的测试图像。
