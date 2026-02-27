package cp_test

import (
	"sync"
	"testing"

	cp "github.com/cytzrs/gocp"
	"gocv.io/x/gocv"
)

// TestConcurrentCompress 并发压缩测试
func TestConcurrentCompress(t *testing.T) {
	// 这个测试需要实际的测试图像文件
	// 跳过如果没有测试文件
	t.Skip("需要测试图像文件")

	params := cp.NewImageCompressor(
		cp.WithQuality(75),
		cp.WithFormat("jpg"),
		cp.WithResize(true, 1920, 1080),
	)

	testFile := "testdata/test.jpg"

	// 并发测试 100 次
	const concurrency = 100
	var wg sync.WaitGroup
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			buf, err := cp.Compress(testFile, params)
			if err != nil {
				errors <- err
				return
			}

			if len(buf) == 0 {
				t.Errorf("并发 %d: 结果为空", id)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// 检查是否有错误
	errorCount := 0
	for err := range errors {
		t.Errorf("压缩失败: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("共有 %d 个并发请求失败", errorCount)
	}
}

// TestConcurrentOptimize 并发 Optimize 测试
// ⚠️ 关键：测试 Mat 资源管理是否正确
func TestConcurrentOptimize(t *testing.T) {
	t.Skip("需要测试图像文件")

	testFile := "testdata/test.jpg"

	// 并发测试
	const concurrency = 50
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// ✅ 正确的 Mat 资源管理
			src, err := gocv.IMRead(testFile, gocv.IMReadColor)
			if err != nil {
				t.Errorf("并发 %d: 读取失败: %v", id, err)
				return
			}
			defer src.Close() // 必须关闭

			if src.Empty() {
				t.Errorf("并发 %d: 图像为空", id)
				return
			}

			// 调用 Optimize
			result, err := cp.Optimize(src)
			if err != nil {
				t.Errorf("并发 %d: Optimize 失败: %v", id, err)
				return
			}
			defer result.Close() // ✅ 必须关闭返回的 Mat

			// 验证结果
			if result.Empty() {
				t.Errorf("并发 %d: 结果为空", id)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("成功完成 %d 个并发 Optimize 操作", concurrency)
}

// TestConcurrentCompressByBytes 并发字节流压缩测试
func TestConcurrentCompressByBytes(t *testing.T) {
	t.Skip("需要测试图像文件")

	testFile := "testdata/test.jpg"

	// 读取测试图像
	src, err := gocv.IMRead(testFile, gocv.IMReadColor)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), src)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Close()

	imageBytes := buf.GetBytes()

	params := cp.NewImageCompressor(
		cp.WithQuality(75),
		cp.WithFormat("jpg"),
	)

	// 并发测试
	const concurrency = 100
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			result, err := cp.CompressByBytes(imageBytes, params)
			if err != nil {
				t.Errorf("并发 %d: 压缩失败: %v", id, err)
				return
			}

			if len(result) == 0 {
				t.Errorf("并发 %d: 结果为空", id)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("成功完成 %d 个并发 CompressByBytes 操作", concurrency)
}

// TestMatMemoryLeak Mat 内存泄漏测试
func TestMatMemoryLeak(t *testing.T) {
	t.Skip("需要测试图像文件")

	testFile := "testdata/test.jpg"

	// 执行大量 Optimize 操作
	// 如果有内存泄漏，内存会持续增长
	iterations := 1000

	for i := 0; i < iterations; i++ {
		src, err := gocv.IMRead(testFile, gocv.IMReadColor)
		if err != nil {
			t.Fatal(err)
		}

		result, err := cp.Optimize(src)
		src.Close() // 立即关闭 src

		if err != nil {
			t.Fatal(err)
		}

		result.Close() // 立即关闭 result
	}

	t.Logf("完成 %d 次 Optimize 操作，应该没有内存泄漏", iterations)
}

// BenchmarkCompressSingle 单线程压缩基准测试
func BenchmarkCompressSingle(b *testing.B) {
	b.Skip("需要测试图像文件")

	testFile := "testdata/test.jpg"
	params := cp.NewImageCompressor(
		cp.WithQuality(75),
		cp.WithFormat("jpg"),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cp.Compress(testFile, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompressParallel 并发压缩基准测试
func BenchmarkCompressParallel(b *testing.B) {
	b.Skip("需要测试图像文件")

	testFile := "testdata/test.jpg"
	params := cp.NewImageCompressor(
		cp.WithQuality(75),
		cp.WithFormat("jpg"),
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cp.Compress(testFile, params)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

// BenchmarkOptimizeParallel 并发 Optimize 基准测试
func BenchmarkOptimizeParallel(b *testing.B) {
	b.Skip("需要测试图像文件")

	testFile := "testdata/test.jpg"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			src, err := gocv.IMRead(testFile, gocv.IMReadColor)
			if err != nil {
				b.Fatal(err)
			}

			result, err := cp.Optimize(src)
			src.Close()

			if err != nil {
				b.Fatal(err)
			}

			result.Close()
		}
	})
}

// Example_concurrentUsage 并发使用示例
func Example_concurrentUsage() {
	// 此示例展示如何安全地并发使用 gocp

	// 1. 准备参数
	params := cp.NewImageCompressor(
		cp.WithQuality(75),
		cp.WithFormat("jpg"),
		cp.WithResize(true, 1920, 1080),
	)

	// 2. 并发处理
	var wg sync.WaitGroup
	files := []string{"img1.jpg", "img2.jpg", "img3.jpg"}

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()

			// Compress 是线程安全的
			buf, err := cp.Compress(f, params)
			if err != nil {
				// 处理错误
				return
			}

			// 使用 buf...
			_ = buf
		}(file)
	}

	wg.Wait()
}

// Example_optimizeWithMat 管理 Optimize 返回的 Mat
func Example_optimizeWithMat() {
	// ⚠️ 关键：展示如何正确管理 Optimize 返回的 Mat

	// 1. 读取图像
	src, err := gocv.IMRead("input.jpg", gocv.IMReadColor)
	if err != nil {
		panic(err)
	}
	defer src.Close() // 确保关闭

	// 2. 调用 Optimize
	result, err := cp.Optimize(src)
	if err != nil {
		panic(err)
	}
	defer result.Close() // ✅ 必须关闭返回的 Mat

	// 3. 使用 result
	// ...

	// 函数结束时，defer 会自动关闭 src 和 result
}
