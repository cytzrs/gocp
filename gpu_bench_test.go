package cp

import (
	"testing"
)

func BenchmarkCPUCompress_4K(b *testing.B) {
	src := mustCreateTestImage(4096, 3072)
	params := NewImageCompressor(
		WithQuality(75),
		WithFormat("jpg"),
		WithResize(true, 3072, 4096),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompressByBytes(src, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGPUCompress_4K(b *testing.B) {
	if !HaveOpenCL() {
		b.Skip("OpenCL not available")
	}

	src := mustCreateTestImage(4096, 3072)
	params := NewImageCompressor(
		WithQuality(75),
		WithFormat("jpg"),
		WithResize(true, 3072, 4096),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompressByBytesWithGPU(src, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCPUCompress_1080p(b *testing.B) {
	src := mustCreateTestImage(1920, 1080)
	params := NewImageCompressor(
		WithQuality(75),
		WithFormat("jpg"),
		WithResize(true, 1280, 720),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompressByBytes(src, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGPUCompress_1080p(b *testing.B) {
	if !HaveOpenCL() {
		b.Skip("OpenCL not available")
	}

	src := mustCreateTestImage(1920, 1080)
	params := NewImageCompressor(
		WithQuality(75),
		WithFormat("jpg"),
		WithResize(true, 1280, 720),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompressByBytesWithGPU(src, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCPUCompress_WebP(b *testing.B) {
	src := mustCreateTestImage(1920, 1080)
	params := NewImageCompressor(
		WithQuality(80),
		WithFormat("webp"),
		WithResize(true, 1280, 720),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompressByBytes(src, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGPUCompress_WebP(b *testing.B) {
	if !HaveOpenCL() {
		b.Skip("OpenCL not available")
	}

	src := mustCreateTestImage(1920, 1080)
	params := NewImageCompressor(
		WithQuality(80),
		WithFormat("webp"),
		WithResize(true, 1280, 720),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompressByBytesWithGPU(src, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}
