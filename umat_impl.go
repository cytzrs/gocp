package cp

/*
#cgo CXXFLAGS: -std=c++11
#cgo linux pkg-config: opencv4

#include <stdlib.h>

// OpenCL control
extern int opencl_haveOpenCL();
extern void opencl_setUseOpenCL(int enable);
extern int opencl_useOpenCL();

// UMat operations
typedef void* UMatHandle;
typedef void* MatHandle;

extern UMatHandle UMat_New();
extern UMatHandle UMat_NewFromMat(MatHandle matPtr);
extern UMatHandle UMat_NewWithSize(int rows, int cols, int type);
extern void UMat_Close(UMatHandle umatPtr);
extern int UMat_Empty(UMatHandle umatPtr);
extern int UMat_Rows(UMatHandle umatPtr);
extern int UMat_Cols(UMatHandle umatPtr);
extern int UMat_Type(UMatHandle umatPtr);
extern UMatHandle UMat_CopyTo(UMatHandle srcPtr);
extern MatHandle UMat_GetMat(UMatHandle umatPtr);
extern void Mat_Close(MatHandle matPtr);
extern int UMat_Resize(UMatHandle srcPtr, UMatHandle* dstPtr, int width, int height, int interpolation);
extern int UMat_GaussianBlur(UMatHandle srcPtr, UMatHandle* dstPtr, int ksizeWidth, int ksizeHeight, double sigmaX, double sigmaY);
extern int UMat_CvtColor(UMatHandle srcPtr, UMatHandle* dstPtr, int code);

// GPU compression pipeline
typedef struct {
    unsigned char* data;
    int length;
    int success;
} GPUCompressResult;

extern GPUCompressResult GPU_CompressImage(
    MatHandle matPtr,
    int doResize, int targetWidth, int targetHeight,
    int blurKsizeW, int blurKsizeH, double sigmaX, double sigmaY,
    const char* ext,
    int* encodeParams, int encodeParamCount);

extern void GPU_FreeResultData(GPUCompressResult* result);
*/
import "C"

import (
	"errors"
	"unsafe"

	"gocv.io/x/gocv"
)

// UMat represents an OpenCV UMat (Unified Mat) that can utilize OpenCL acceleration.
type UMat struct {
	p C.UMatHandle
}

// NewUMat creates a new empty UMat.
func NewUMat() UMat {
	return UMat{p: C.UMat_New()}
}

// NewUMatFromMat creates a UMat from a regular Mat (uploads to GPU if OpenCL enabled).
func NewUMatFromMat(mat gocv.Mat) UMat {
	return UMat{p: C.UMat_NewFromMat(C.MatHandle(unsafe.Pointer(mat.Ptr())))}
}

// NewUMatWithSize creates a new UMat with specified size and type.
func NewUMatWithSize(rows, cols int, matType gocv.MatType) UMat {
	return UMat{p: C.UMat_NewWithSize(C.int(rows), C.int(cols), C.int(matType))}
}

// Close releases the UMat resources.
func (u *UMat) Close() error {
	if u.p != nil {
		C.UMat_Close(u.p)
		u.p = nil
	}
	return nil
}

// Empty returns true if the UMat has no elements.
func (u *UMat) Empty() bool {
	return C.UMat_Empty(u.p) != 0
}

// Rows returns the number of rows.
func (u *UMat) Rows() int {
	return int(C.UMat_Rows(u.p))
}

// Cols returns the number of columns.
func (u *UMat) Cols() int {
	return int(C.UMat_Cols(u.p))
}

// Type returns the element type.
func (u *UMat) Type() gocv.MatType {
	return gocv.MatType(C.UMat_Type(u.p))
}

// CopyTo creates a copy of the UMat.
func (u *UMat) CopyTo() UMat {
	return UMat{p: C.UMat_CopyTo(u.p)}
}

// Resize resizes the UMat using GPU acceleration.
func (u *UMat) Resize(width, height int, interpolation gocv.InterpolationFlags) (UMat, error) {
	var dstPtr C.UMatHandle
	ret := C.UMat_Resize(u.p, &dstPtr, C.int(width), C.int(height), C.int(interpolation))
	if ret != 0 {
		return UMat{}, errors.New("UMat resize failed")
	}
	return UMat{p: dstPtr}, nil
}

// GaussianBlur applies Gaussian blur using GPU acceleration.
func (u *UMat) GaussianBlur(ksizeWidth, ksizeHeight int, sigmaX, sigmaY float64) (UMat, error) {
	var dstPtr C.UMatHandle
	ret := C.UMat_GaussianBlur(u.p, &dstPtr, C.int(ksizeWidth), C.int(ksizeHeight), C.double(sigmaX), C.double(sigmaY))
	if ret != 0 {
		return UMat{}, errors.New("UMat GaussianBlur failed")
	}
	return UMat{p: dstPtr}, nil
}

// CvtColor converts color space using GPU acceleration.
func (u *UMat) CvtColor(code gocv.ColorConversionCode) (UMat, error) {
	var dstPtr C.UMatHandle
	ret := C.UMat_CvtColor(u.p, &dstPtr, C.int(code))
	if ret != 0 {
		return UMat{}, errors.New("UMat CvtColor failed")
	}
	return UMat{p: dstPtr}, nil
}

// OpenCL control functions

// UseOpenCL enables or disables OpenCL acceleration globally.
func UseOpenCL(enable bool) {
	val := 0
	if enable {
		val = 1
	}
	C.opencl_setUseOpenCL(C.int(val))
}

// HaveOpenCL checks if OpenCL is available on the system.
func HaveOpenCL() bool {
	return C.opencl_haveOpenCL() != 0
}

// UsingOpenCL checks if OpenCL is currently enabled.
func UsingOpenCL() bool {
	return C.opencl_useOpenCL() != 0
}

// CompressWithGPU compresses an image using OpenCL GPU acceleration.
// Falls back to CPU if OpenCL is not available.
func CompressWithGPU(imgPath string, params *ImageCompressor) ([]byte, error) {
	if !HaveOpenCL() {
		return Compress(imgPath, params)
	}
	UseOpenCL(true)

	img := gocv.IMRead(imgPath, gocv.IMReadColor)
	if img.Empty() {
		return nil, errors.New("failed to read image")
	}
	defer img.Close()

	return gpuCompress(img, params)
}

// CompressByBytesWithGPU compresses image bytes using OpenCL GPU acceleration.
// Falls back to CPU if OpenCL is not available.
func CompressByBytesWithGPU(src []byte, params *ImageCompressor) ([]byte, error) {
	if !HaveOpenCL() {
		return CompressByBytes(src, params)
	}
	UseOpenCL(true)

	img, err := gocv.IMDecode(src, gocv.IMReadColor)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	if img.Empty() {
		return nil, errors.New("failed to decode image")
	}

	return gpuCompress(img, params)
}

// gpuCompress runs the full GPU pipeline:
// Mat -> UMat upload -> Resize -> GaussianBlur -> Download -> Encode
func gpuCompress(mat gocv.Mat, params *ImageCompressor) ([]byte, error) {
	ext := ".jpg"
	cParams := []C.int{
		C.int(gocv.IMWriteJpegQuality), C.int(params.Quality),
		C.int(gocv.IMWriteJpegOptimize), 1,
		C.int(gocv.IMWriteJpegChromaQuality), C.int(params.Quality),
	}
	if params.Format == "webp" {
		ext = ".webp"
		cParams = []C.int{C.int(gocv.IMWriteWebpQuality), C.int(params.Quality)}
	}

	cExt := C.CString(ext)
	defer C.free(unsafe.Pointer(cExt))

	result := C.GPU_CompressImage(
		C.MatHandle(unsafe.Pointer(mat.Ptr())),
		cboolToInt(params.Resize), C.int(params.Width), C.int(params.Height),
		3, 3, C.double(1.0), C.double(1.0),
		cExt,
		&cParams[0], C.int(len(cParams)),
	)
	defer C.GPU_FreeResultData(&result)

	if result.success != 0 {
		return nil, errors.New("GPU compression pipeline failed")
	}

	// Copy C buffer to Go heap
	goBytes := C.GoBytes(unsafe.Pointer(result.data), C.int(result.length))
	return goBytes, nil
}

func cboolToInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
