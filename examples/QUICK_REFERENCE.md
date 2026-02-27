# 并发调用 gocp 快速参考卡片

## 🚨 最关键的规则

### Mat 资源管理（第一优先级）

```go
// ✅ 正确模式
result, err := cp.Optimize(src)
if err != nil {
    return err
}
defer result.Close()  // 必须！

// ❌ 错误模式
result, _ := cp.Optimize(src)
// 忘记 Close() → 内存泄漏！
```

**规则**：任何返回 `gocv.Mat` 的函数，调用者**必须**负责 `Close()`

---

## 📋 API 函数分类

### ✅ 不需要手动管理（返回 []byte）

| 函数 | 返回值 | 调用者责任 |
|------|--------|-----------|
| `Compress` | `[]byte` | ❌ 不需要（Go GC 管理） |
| `CompressByBytes` | `[]byte` | ❌ 不需要（Go GC 管理） |

```go
buf, err := cp.Compress("input.jpg", params)
// buf 由 Go GC 管理，无需手动释放
```

### ⚠️ 需要手动管理（返回 gocv.Mat）

| 函数 | 返回值 | 调用者责任 |
|------|--------|-----------|
| `Optimize` | `gocv.Mat` | ✅ **必须 defer Close()** |

```go
result, err := cp.Optimize(src)
defer result.Close()  // 必须这样做！
```

---

## 🔥 并发模式模板

### 模式1: 简单并发（Compress）

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()

        buf, err := cp.Compress(files[id], params)
        if err != nil {
            log.Printf("失败: %v", err)
            return
        }
        // 使用 buf...
    }(i)
}
wg.Wait()
```

### 模式2: 协程池（推荐）

```go
import "github.com/panjf2000/ants/v2"

pool, _ := ants.NewPool(10, ants.WithExpiryDuration(30*time.Second))
defer pool.Release()

for _, file := range files {
    f := file // 避免闭包问题
    pool.Submit(func() {
        buf, err := cp.Compress(f, params)
        // 处理结果...
    })
}
```

### 模式3: 使用 Optimize（最复杂）

```go
go func() {
    // 1. 打开源图像
    src, err := gocv.IMRead(path, gocv.IMReadColor)
    if err != nil {
        return
    }
    defer src.Close()  // ✅ 关闭源

    // 2. 调用 Optimize
    result, err := cp.Optimize(src)
    if err != nil {
        return
    }
    defer result.Close()  // ✅ 关闭结果

    // 3. 使用 result
    gocv.IMWrite("output.jpg", result)
}()
```

---

## ⚡ 并发数建议

| 场景 | 并发数 | 超时 | 说明 |
|------|--------|------|------|
| 小图压缩 | 10-20 | 10s | 吞吐量优先 |
| 大图压缩 | 5-10 | 30s | 平衡性能 |
| 使用 Optimize | 3-5 | 60s | Optimize 耗时 |
| HTTP API | 5-10 | 30s | 稳定性优先 |

---

## 🐛 常见错误

### 错误1: 忘记关闭 Mat

```go
// ❌ 错误
result, _ := cp.Optimize(src)
// 内存泄漏！

// ✅ 正确
result, _ := cp.Optimize(src)
defer result.Close()
```

### 错误2: 闭包捕获循环变量

```go
// ❌ 错误
for i := 0; i < 10; i++ {
    go func() {
        fmt.Println(i) // 可能打印 10 个 10
    }()
}

// ✅ 正确
for i := 0; i < 10; i++ {
    i := i // 创建局部变量
    go func() {
        fmt.Println(i) // 正确打印 0-9
    }()
}
```

### 错误3: 共享 Mat 对象

```go
// ❌ 错误：多个 goroutine 使用同一个 Mat
var mat gocv.Mat
for i := 0; i < 10; i++ {
    go func() {
        cp.Optimize(mat) // 数据竞争！
    }()
}

// ✅ 正确：每个 goroutine 使用独立的 Mat
for i := 0; i < 10; i++ {
    go func() {
        mat, _ := gocv.IMRead("file.jpg", gocv.IMReadColor)
        defer mat.Close()
        cp.Optimize(mat) // 安全
    }()
}
```

---

## 🔍 调试检查清单

并发调用前的检查：

- [ ] 是否使用了协程池限制并发？
- [ ] 所有 `gocv.Mat` 都有 `defer Close()` 吗？
- [ ] 是否避免了闭包捕获循环变量？
- [ ] 是否处理了所有错误？
- [ ] 是否设置了超时时间？
- [ ] 是否有并发安全的统计/日志？

---

## 📦 完整示例

```go
package main

import (
    "log"
    "sync"
    "time"

    cp "github.com/cytzrs/gocp"
    "github.com/panjf2000/ants/v2"
)

func main() {
    // 1. 准备参数
    params := cp.NewImageCompressor(
        cp.WithQuality(75),
        cp.WithFormat("jpg"),
    )

    // 2. 创建协程池
    pool, err := ants.NewPool(10, ants.WithExpiryDuration(30*time.Second))
    if err != nil {
        panic(err)
    }
    defer pool.Release()

    // 3. 准备文件列表
    files := []string{"img1.jpg", "img2.jpg", "img3.jpg"}

    // 4. 并发处理
    var wg sync.WaitGroup
    for _, file := range files {
        wg.Add(1)
        f := file // ✅ 避免闭包问题
        pool.Submit(func() {
            defer wg.Done()

            buf, err := cp.Compress(f, params)
            if err != nil {
                log.Printf("❌ %s: %v", f, err)
                return
            }

            log.Printf("✅ %s: %d bytes", f, len(buf))
        })
    }

    // 5. 等待完成
    wg.Wait()
    log.Println("所有任务完成")
}
```

---

## 🆘 遇到问题？

### 内存持续增长
→ 检查是否有 Mat 忘记 `Close()`

### CPU 使用率低但请求堆积
→ 增加协程池并发数

### 偶尔崩溃
→ 检查是否有共享 Mat 或 double free

### 处理速度慢
→ 检查图像大小、调整并发数、减少 Optimize 使用

---

## 📚 更多资源

- 完整示例：`examples/README.md`
- API 文档：`CLAUDE.md`
- 并发测试：`concurrent_test.go`
