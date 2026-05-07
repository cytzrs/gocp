package cp

import (
	"os"
	"testing"

	"gocv.io/x/gocv"
)

func TestOpenCLDetection(t *testing.T) {
	hasOpenCL := HaveOpenCL()
	t.Logf("OpenCL available: %v", hasOpenCL)

	UseOpenCL(true)
	t.Logf("OpenCL enabled: %v", UsingOpenCL())

	umat := NewUMat()
	defer umat.Close()
	if !umat.Empty() {
		t.Fatal("new UMat should be empty")
	}

	mat := gocv.NewMatWithSize(100, 100, gocv.MatTypeCV8UC3)
	defer mat.Close()

	umatFromMat := NewUMatFromMat(mat)
	defer umatFromMat.Close()

	if umatFromMat.Empty() {
		t.Fatal("UMat from Mat should not be empty")
	}
	if umatFromMat.Rows() != 100 || umatFromMat.Cols() != 100 {
		t.Fatalf("UMat dimensions mismatch: got %dx%d, want 100x100",
			umatFromMat.Rows(), umatFromMat.Cols())
	}
}

func TestGPUCompressionCorrectness(t *testing.T) {
	if !HaveOpenCL() {
		t.Skip("OpenCL not available, skipping GPU compression test")
	}

	src := mustCreateTestImage(800, 600)
	params := NewImageCompressor(
		WithQuality(75),
		WithFormat("jpg"),
		WithResize(true, 400, 300),
	)

	cpuResult, err := CompressByBytes(src, params)
	if err != nil {
		t.Fatalf("CPU compression failed: %v", err)
	}

	gpuResult, err := CompressByBytesWithGPU(src, params)
	if err != nil {
		t.Fatalf("GPU compression failed: %v", err)
	}

	if len(cpuResult) == 0 {
		t.Fatal("CPU result is empty")
	}
	if len(gpuResult) == 0 {
		t.Fatal("GPU result is empty")
	}

	t.Logf("CPU output: %d bytes, GPU output: %d bytes", len(cpuResult), len(gpuResult))
}

func TestGPUCompressionByPath(t *testing.T) {
	if !HaveOpenCL() {
		t.Skip("OpenCL not available")
	}

	// Create temp image file
	src := mustCreateTestImage(640, 480)
	tmpFile, err := os.CreateTemp("", "gocp_test_*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(src)
	tmpFile.Close()

	params := NewImageCompressor(
		WithQuality(80),
		WithFormat("jpg"),
	)

	gpuResult, err := CompressWithGPU(tmpFile.Name(), params)
	if err != nil {
		t.Fatalf("CompressWithGPU failed: %v", err)
	}
	if len(gpuResult) == 0 {
		t.Fatal("GPU result is empty")
	}
	t.Logf("GPU path compress: %d bytes from %s", len(gpuResult), tmpFile.Name())
}

func mustCreateTestImage(width, height int) []byte {
	img := gocv.NewMatWithSize(height, width, gocv.MatTypeCV8UC3)
	defer img.Close()
	img.SetTo(gocv.NewScalar(128, 64, 192, 0))

	buf, err := gocv.IMEncode(gocv.JPEGFileExt, img)
	if err != nil {
		panic("failed to encode test image: " + err.Error())
	}

	cBytes := buf.GetBytes()
	result := make([]byte, len(cBytes))
	copy(result, cBytes)
	buf.Close()
	return result
}
