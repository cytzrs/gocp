# 并发调用最佳实践

本目录包含 gocp 库的第三方并发调用示例，展示了如何安全、高效地在生产环境中使用该库。

## 📁 示例文件

### 1. `concurrent_usage_test.go`
全面的并发使用示例，包括：
- **批量图像处理**：使用协程池处理大量文件
- **HTTP 图像处理服务**：完整的 API 服务实现
- **流水线处理**：多阶段图像处理管道

### 2. `http_api_example.go`
简化的 HTTP API 服务器示例：
- RESTful API 设计
- 并发安全的请求处理
- 统计和监控
- 详细的资源管理注释

## 🚀 快速开始

### 场景1: 批量处理本地文件

```go
package main

import (
    "fmt"
    "log"
    "time"

    cp "github.com/cytzrs/gocp"
    "github.com/panjf2000/ants/v2"
)

func main() {
    // 准备参数
    params := cp.NewImageCompressor(
        cp.WithQuality(75),
        cp.WithFormat("jpg"),
        cp.WithResize(true, 1920, 1080),
    )

    // 创建协程池（控制并发数）
    pool, err := ants.NewPool(10, ants.WithExpiryDuration(30*time.Second))
    if err != nil {
        panic(err)
    }
    defer pool.Release()

    files := []string{"image1.jpg", "image2.jpg", "image3.jpg"}

    for _, file := range files {
        f := file // 避免闭包问题
        pool.Submit(func() {
            buf, err := cp.Compress(f, params)
            if err != nil {
                log.Printf("处理 %s 失败: %v", f, err)
                return
            }
            log.Printf("处理 %s 成功，输出 %d 字节", f, len(buf))
        })
    }

    // 等待所有任务完成...
    time.Sleep(10 * time.Second)
}
```

### 场景2: HTTP API 服务

```bash
# 启动服务器
go run examples/http_api_example.go

# 客户端调用 - 压缩图像
curl -X POST \
  -F "image=@input.jpg" \
  -F "quality=80" \
  -F "format=jpg" \
  http://localhost:8080/compress \
  --output output.jpg

# 查看服务统计
curl http://localhost:8080/stats
```

### 场景3: 使用 Optimize（绿色背景移除）

```go
// ⚠️⚠️⚠️ 关键：必须正确管理 Mat 资源
func processWithBgRemoval(imageData []byte) ([]byte, error) {
    // 1. 解码
    src, err := gocv.IMDecode(imageData, gocv.IMReadColor)
    if err != nil {
        return nil, err
    }
    defer src.Close() // ✅ 必须关闭

    // 2. 移除绿色背景
    optimized, err := cp.Optimize(src)
    if err != nil {
        return nil, err
    }
    defer optimized.Close() // ✅ 必须关闭（Optimize 返回的 Mat）

    // 3. 编码
    buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), optimized)
    if err != nil {
        return nil, err
    }
    defer buf.Close() // ✅ 必须关闭

    return buf.GetBytes(), nil
}
```

## ⚠️ 关键注意事项

### 1. Mat 资源管理（最重要！）

```go
// ❌ 错误：内存泄漏
result, _ := cp.Optimize(src)
// 忘记 defer result.Close()

// ✅ 正确：使用 defer 确保释放
result, err := cp.Optimize(src)
if err != nil {
    return err
}
defer result.Close() // 必须这样做！

// 使用 result...
```

### 2. 闭包变量捕获

```go
// ❌ 错误：所有 goroutine 使用相同的循环变量
for i := 0; i < 10; i++ {
    go func() {
        fmt.Println(i) // 可能打印 10 个 10
    }()
}

// ✅ 正确：使用局部变量
for i := 0; i < 10; i++ {
    i := i // 创建局部变量
    go func() {
        fmt.Println(i) // 正确打印 0-9
    }()
}
```

### 3. 协程池配置

| 场景 | 推荐并发数 | 超时时间 | 说明 |
|------|-----------|---------|------|
| 小图像 (<1MB) | 10-20 | 10s | 高吞吐量 |
| 中等图像 (1-5MB) | 5-10 | 30s | 平衡性能 |
| 大图像 (>5MB) | 3-5 | 60s | 避免OOM |
| 使用 Optimize | 减半 | 60s | Optimize 耗时更长 |

### 4. 错误处理

```go
// ✅ 完整的错误处理
buf, err := cp.Compress(path, params)
if err != nil {
    // 记录错误
    log.Printf("压缩失败: %v", err)
    // 返回错误或重试
    return err
}

if buf == nil {
    log.Printf("压缩结果为空")
    return errors.New("压缩结果为空")
}
```

## 📊 性能优化建议

### 1. 控制并发数

```go
// 根据图像大小动态调整
func getConcurrency(avgImageSizeMB float64) int {
    switch {
    case avgImageSizeMB < 1:
        return 20 // 小图，高并发
    case avgImageSizeMB < 5:
        return 10 // 中图，中并发
    default:
        return 5  // 大图，低并发
    }
}
```

### 2. 监控和统计

```go
// 使用 atomic 包进行并发安全的计数
var (
    successCount atomic.Int64
    failureCount atomic.Int64
)

successCount.Add(1)
log.Printf("成功: %d, 失败: %d",
    successCount.Load(),
    failureCount.Load())
```

### 3. 资源清理

```go
type Service struct {
    pool *ants.Pool
}

func (s *Service) Close() {
    s.pool.Release() // 释放协程池
}

// 使用 defer 确保清理
service := NewService()
defer service.Close()
```

## 🧪 测试建议

```go
func TestConcurrentCompress(t *testing.T) {
    params := cp.NewImageCompressor(
        cp.WithQuality(75),
        cp.WithFormat("jpg"),
    )

    // 并发测试
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            buf, err := cp.Compress("test.jpg", params)
            if err != nil {
                t.Errorf("并发 %d 失败: %v", id, err)
                return
            }

            if len(buf) == 0 {
                t.Errorf("并发 %d 结果为空", id)
            }
        }(i)
    }
    wg.Wait()
}
```

## 🔍 故障排查

### 问题1: 内存持续增长

**原因**: 忘记关闭 Mat 对象
**解决**: 检查所有 `gocv.Mat` 是否都有 `defer Close()`

### 问题2: CPU 使用率低但请求堆积

**原因**: 协程池配置过小
**解决**: 增加并发数或检查是否有慢任务

### 问题3: 偶尔的崩溃

**原因**: 并发访问共享数据或 double free
**解决**:
- 检查是否有共享的 Mat 对象
- 确保每个 goroutine 使用独立的 Mat

### 问题4: 处理速度慢

**原因**:
- Optimize 操作耗时较长
- 图像尺寸过大
- 并发配置不合理

**解决**:
- 对 Optimize 使用单独的协程池（更小的并发数）
- 考虑预处理（缩小尺寸）
- 调整并发参数

## 📚 相关资源

- [ants 协程池文档](https://github.com/panjf2000/ants)
- [GoCV 官方文档](https://gocv.io/)
- [Go 并发编程最佳实践](https://go.dev/doc/effective_go#concurrency)

## 🤝 贡献

欢迎提交问题和改进建议！

## 📄 许可证

与主项目相同
