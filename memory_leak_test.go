package cp

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/panjf2000/ants/v2"
	"gocv.io/x/gocv"
)

// TestMemoryLeakDetection 测试循环调用 CompressByBytes 是否有内存泄露
func TestMemoryLeakDetection(t *testing.T) {
	// 读取测试图片
	testData, err := loadTestImage()
	if err != nil {
		t.Skip("跳过测试：无法加载测试图片", err)
		return
	}

	// 强制 GC，获取初始内存
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	t.Logf("初始内存: Stack=%dMB, Heap=%dMB", m1.StackInuse/1024/1024, m1.HeapInuse/1024/1024)

	// 循环压缩 1000 次
	iterations := 1000
	for i := 0; i < iterations; i++ {
		_, err := CompressByBytes(testData, &ImageCompressor{
			Format:  "jpg",
			Quality: 25,
			Resize:  true,
			Height:  1024,
			Width:   1024,
		})
		if err != nil {
			t.Logf("第 %d 次压缩失败: %v", i, err)
		}

		// 每 100 次检查一次
		if (i+1)%100 == 0 {
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			stackGrowth := int64(m2.StackInuse) - int64(m1.StackInuse)
			heapGrowth := int64(m2.HeapInuse) - int64(m1.HeapInuse)

			t.Logf("第 %d 次: Stack=%dMB(增长%.1fMB), Heap=%dMB(增长%.1fMB)",
				i+1,
				m2.StackInuse/1024/1024,
				float64(stackGrowth)/1024/1024,
				m2.HeapInuse/1024/1024,
				float64(heapGrowth)/1024/1024)

			// 如果 Stack 增长超过 10MB，可能是 gocv 的 Mat 泄露
			if stackGrowth > 10*1024*1024 {
				t.Errorf("检测到 Stack 内存泄露！增长 %.1fMB", float64(stackGrowth)/1024/1024)
			}
		}
	}

	// 最终检查
	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)

	stackGrowth := int64(m3.StackInuse) - int64(m1.StackInuse)
	heapGrowth := int64(m3.HeapInuse) - int64(m1.HeapInuse)

	t.Logf("最终结果: Stack=%dMB(增长%.1fMB), Heap=%dMB(增长%.1fMB)",
		m3.StackInuse/1024/1024, float64(stackGrowth)/1024/1024,
		m3.HeapInuse/1024/1024, float64(heapGrowth)/1024/1024)

	if stackGrowth > 20*1024*1024 {
		t.Errorf("严重的 Stack 内存泄露！增长 %.1fMB", float64(stackGrowth)/1024/1024)
	}
}

// TestMemoryLeakDetectionParallel 并发测试内存泄露
func TestMemoryLeakDetectionParallel(t *testing.T) {
	testData, err := loadTestImage()
	if err != nil {
		t.Skip("跳过测试：无法加载测试图片", err)
		return
	}

	// 创建线程池（模拟 ZUIG 的使用方式）
	pool, err := ants.NewPool(16, ants.WithPreAlloc(false))
	if err != nil {
		t.Fatal("创建线程池失败:", err)
	}
	defer pool.Release()

	// 强制 GC，获取初始内存
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	t.Logf("初始内存: Stack=%dMB, Heap=%dMB", m1.StackInuse/1024/1024, m1.HeapInuse/1024/1024)

	// 并发压缩 1000 次
	iterations := 1000
	for i := 0; i < iterations; i++ {
		idx := i
		pool.Submit(func() {
			_, err := CompressByBytes(testData, &ImageCompressor{
				Format:  "jpg",
				Quality: 25,
				Resize:  true,
				Height:  1024,
				Width:   1024,
			})
			if err != nil && idx%100 == 0 {
				t.Logf("任务 %d 压缩失败: %v", idx, err)
			}
		})
	}

	// 等待所有任务完成
	for pool.Running() > 0 {
		runtime.GC()
	}

	// 最终检查
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	stackGrowth := int64(m2.StackInuse) - int64(m1.StackInuse)
	heapGrowth := int64(m2.HeapInuse) - int64(m1.HeapInuse)

	t.Logf("并行测试最终: Stack=%dMB(增长%.1fMB), Heap=%dMB(增长%.1fMB)",
		m2.StackInuse/1024/1024, float64(stackGrowth)/1024/1024,
		m2.HeapInuse/1024/1024, float64(heapGrowth)/1024/1024)

	if stackGrowth > 50*1024*1024 {
		t.Errorf("严重的并发 Stack 内存泄露！增长 %.1fMB", float64(stackGrowth)/1024/1024)
	}
}

// 辅助函数：加载测试图片
func loadTestImage() ([]byte, error) {
	// 创建一个测试图片
	mat := gocv.NewMatWithSize(800, 600, gocv.MatTypeCV8UC3)
	defer mat.Close()

	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), mat)
	if err != nil {
		return nil, fmt.Errorf("无法创建测试图片: %w", err)
	}
	defer buf.Close()

	data := buf.GetBytes()
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}
