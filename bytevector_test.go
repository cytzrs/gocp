package cp_test

import (
	"runtime"
	"testing"
	"unsafe"

	"gocv.io/x/gocv"
)

// TestByteVectorGetBytesBehavior 测试 ByteVector.GetBytes() 的行为
//
// 这个测试验证：GetBytes() 返回的 []byte 是否依赖原始的 ByteVector
//
// ⚠️ 关键发现：GetBytes() 返回的是 C 内存的视图（view），不是副本（copy）！
// 这是 use-after-free 漏洞的根源。
func TestByteVectorGetBytesBehavior(t *testing.T) {
	// 创建测试图像
	mat := gocv.NewMatWithSize(100, 100, gocv.MatTypeCV8UC3)
	defer mat.Close()

	// 编码为 ByteVector
	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), mat)
	if err != nil {
		t.Fatal(err)
	}

	// 获取字节切片 - 这是 C 内存的视图！
	_ = buf.GetBytes()

	// 保存字节切片的指针
	bytes := buf.GetBytes()
	bytesPtr := unsafe.SliceData(bytes)
	bytesLen := len(bytes)

	t.Logf("GetBytes() 返回的 []byte: 长度=%d, 指针=%p", bytesLen, bytesPtr)

	// 关闭 ByteVector - 释放 C 内存
	buf.Close()

	// ⚠️ 现在 bytes 是悬空指针！
	// 在 Release 模式下可能仍然"可用"（内存未覆盖）
	// 但这是未定义行为（UB），任何时刻都可能崩溃

	t.Logf("关闭 ByteVector 后，[]byte 长度仍为: %d (但内存已释放！)", len(bytes))

	// 在实际应用中，访问已释放的内存是危险的！
	// 这个测试只是验证行为，不应依赖它
	t.Log("⚠️ 警告：GetBytes() 返回的是 C 内存的视图，Close() 后访问是未定义行为")
}

// TestDeferAndGetBytesOrder 测试 defer 和 GetBytes 的执行顺序
//
// ⚠️ 原测试有误导性！defer Close() 后返回 GetBytes() 是危险的！
func TestDeferAndGetBytesOrder(t *testing.T) {
	mat := gocv.NewMatWithSize(100, 100, gocv.MatTypeCV8UC3)
	defer mat.Close()

	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), mat)
	if err != nil {
		t.Fatal(err)
	}

	// ❌ 错误模式：defer Close() 后返回 GetBytes()
	dangerousResult := func() []byte {
		defer buf.Close()     // 函数返回时执行
		return buf.GetBytes() // 返回 C 内存的视图
	}()

	// ⚠️ dangerousResult 指向已释放的内存！
	// 只是测试"能运行"不代表正确！
	if len(dangerousResult) == 0 {
		t.Fatal("返回的 []byte 为空")
	}

	t.Logf("⚠️ defer Close() 后返回 GetBytes() 长度=%d (但内存已释放！)", len(dangerousResult))

	// 强制 GC
	runtime.GC()

	// ❌ 错误测试：访问已释放的内存是 UB，不应作为"正确"的证明
	t.Log("⚠️ 警告：此测试只是演示行为，不代表这是安全的！")
}

// TestCorrectCopyPattern 测试正确的复制模式
//
// ✅ 安全模式：先复制到 Go 堆，再释放 C 内存
func TestCorrectCopyPattern(t *testing.T) {
	mat := gocv.NewMatWithSize(100, 100, gocv.MatTypeCV8UC3)
	defer mat.Close()

	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), mat)
	if err != nil {
		t.Fatal(err)
	}

	// ✅ 正确模式：先复制，再关闭
	safeResult := func() []byte {
		cBytes := buf.GetBytes() // 获取 C 内存视图
		result := make([]byte, len(cBytes))
		copy(result, cBytes) // 复制到 Go 堆
		buf.Close()          // 现在可以安全释放 C 内存
		return result        // 返回 Go 堆上的副本
	}()

	// ✅ safeResult 是独立的 Go 内存，安全可靠
	if len(safeResult) == 0 {
		t.Fatal("返回的 []byte 为空")
	}

	t.Logf("✅ 正确复制模式返回长度=%d", len(safeResult))

	// 强制 GC，确保 Go 内存正常
	runtime.GC()

	// ✅ 现在访问是安全的
	for i := 0; i < min(10, len(safeResult)); i++ {
		_ = safeResult[i]
	}

	t.Log("✅ 复制后访问安全，数据在 Go 堆上")
}

// BenchmarkDeferAndGetBytes 性能基准测试
//
// ⚠️ 旧的基准测试有误导性，已更新为正确模式
func BenchmarkDeferAndGetBytes(b *testing.B) {
	mat := gocv.NewMatWithSize(1920, 1080, gocv.MatTypeCV8UC3)
	defer mat.Close()

	b.Run("Wrong_DeferAfterReturn", func(b *testing.B) {
		// ❌ 错误模式：defer Close() 后返回 GetBytes()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), mat)
			if err != nil {
				b.Fatal(err)
			}

			result := func() []byte {
				defer buf.Close() // 危险！返回悬空指针
				return buf.GetBytes()
			}()

			// 确保编译器不会优化掉
			_ = result
			// ⚠️ 未定义行为：可能崩溃、数据损坏
		}
	})

	b.Run("Correct_CopyBeforeClose", func(b *testing.B) {
		// ✅ 正确模式：先复制，再关闭
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), mat)
			if err != nil {
				b.Fatal(err)
			}

			cBytes := buf.GetBytes()
			result := make([]byte, len(cBytes))
			copy(result, cBytes)
			buf.Close()

			// 确保编译器不会优化掉
			_ = result
		}
	})
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
