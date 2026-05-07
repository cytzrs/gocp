// OpenCL wrapper for gocp
// C interface to OpenCV's OpenCL (UMat) functionality + GPU compression pipeline

#include <opencv2/core.hpp>
#include <opencv2/core/ocl.hpp>
#include <opencv2/imgproc.hpp>
#include <opencv2/imgcodecs.hpp>
#include <algorithm>
#include <cstring>
#include <stdlib.h>
#include <vector>

extern "C" {

// ============================================================================
// OpenCL control
// ============================================================================

int opencl_haveOpenCL() {
    return cv::ocl::haveOpenCL() ? 1 : 0;
}

void opencl_setUseOpenCL(int enable) {
    cv::ocl::setUseOpenCL(enable != 0);
}

int opencl_useOpenCL() {
    return cv::ocl::useOpenCL() ? 1 : 0;
}

// ============================================================================
// UMat operations
// ============================================================================

typedef void* UMatHandle;
typedef void* MatHandle;

UMatHandle UMat_New() {
    cv::UMat* m = new cv::UMat();
    return (UMatHandle)m;
}

UMatHandle UMat_NewFromMat(void* matPtr) {
    cv::Mat* mat = (cv::Mat*)matPtr;
    cv::UMat* umat = new cv::UMat();
    mat->copyTo(*umat);
    return (UMatHandle)umat;
}

UMatHandle UMat_NewWithSize(int rows, int cols, int type) {
    cv::UMat* m = new cv::UMat(rows, cols, type);
    return (UMatHandle)m;
}

void UMat_Close(UMatHandle umatPtr) {
    cv::UMat* umat = (cv::UMat*)umatPtr;
    delete umat;
}

int UMat_Empty(UMatHandle umatPtr) {
    cv::UMat* umat = (cv::UMat*)umatPtr;
    return umat->empty() ? 1 : 0;
}

int UMat_Rows(UMatHandle umatPtr) {
    cv::UMat* umat = (cv::UMat*)umatPtr;
    return umat->rows;
}

int UMat_Cols(UMatHandle umatPtr) {
    cv::UMat* umat = (cv::UMat*)umatPtr;
    return umat->cols;
}

int UMat_Type(UMatHandle umatPtr) {
    cv::UMat* umat = (cv::UMat*)umatPtr;
    return umat->type();
}

UMatHandle UMat_CopyTo(UMatHandle srcPtr) {
    cv::UMat* src = (cv::UMat*)srcPtr;
    cv::UMat* dst = new cv::UMat();
    src->copyTo(*dst);
    return (UMatHandle)dst;
}

void* UMat_GetMat(UMatHandle umatPtr) {
    cv::UMat* umat = (cv::UMat*)umatPtr;
    cv::Mat* mat = new cv::Mat(umat->getMat(cv::ACCESS_READ));
    return (void*)mat;
}

int UMat_Resize(UMatHandle srcPtr, UMatHandle* dstPtr, int width, int height, int interpolation) {
    try {
        cv::UMat* src = (cv::UMat*)srcPtr;
        cv::UMat* dst = new cv::UMat();
        cv::resize(*src, *dst, cv::Size(width, height), 0, 0, interpolation);
        *dstPtr = (UMatHandle)dst;
        return 0;
    } catch (...) {
        return -1;
    }
}

int UMat_GaussianBlur(UMatHandle srcPtr, UMatHandle* dstPtr, int ksizeWidth, int ksizeHeight, double sigmaX, double sigmaY) {
    try {
        cv::UMat* src = (cv::UMat*)srcPtr;
        cv::UMat* dst = new cv::UMat();
        cv::GaussianBlur(*src, *dst, cv::Size(ksizeWidth, ksizeHeight), sigmaX, sigmaY);
        *dstPtr = (UMatHandle)dst;
        return 0;
    } catch (...) {
        return -1;
    }
}

int UMat_CvtColor(UMatHandle srcPtr, UMatHandle* dstPtr, int code) {
    try {
        cv::UMat* src = (cv::UMat*)srcPtr;
        cv::UMat* dst = new cv::UMat();
        cv::cvtColor(*src, *dst, code);
        *dstPtr = (UMatHandle)dst;
        return 0;
    } catch (...) {
        return -1;
    }
}

// ============================================================================
// GPU Compression Pipeline
// Mat -> UMat upload -> Resize(UMat) -> GaussianBlur(UMat) -> Download -> Encode
// ============================================================================

typedef struct {
    unsigned char* data;
    int length;
    int success;
} GPUCompressResult;

GPUCompressResult GPU_CompressImage(
    MatHandle matPtr,
    int doResize, int targetWidth, int targetHeight,
    int blurKsizeW, int blurKsizeH, double sigmaX, double sigmaY,
    const char* ext,
    int* encodeParams, int encodeParamCount) {

    GPUCompressResult result = {NULL, 0, -1};

    try {
        cv::Mat* inputMat = (cv::Mat*)matPtr;

        // Upload Mat -> UMat (triggers OpenCL transfer if enabled)
        cv::UMat umat;
        inputMat->copyTo(umat);

        // Resize on GPU (if enabled, only shrink)
        if (doResize) {
            int newW = std::min(inputMat->cols, targetWidth);
            int newH = std::min(inputMat->rows, targetHeight);
            cv::UMat resized;
            cv::resize(umat, resized, cv::Size(newW, newH), 0, 0, cv::INTER_AREA);
            umat = resized;
        }

        // GaussianBlur on GPU
        cv::UMat blurred;
        cv::GaussianBlur(umat, blurred, cv::Size(blurKsizeW, blurKsizeH), sigmaX, sigmaY);

        // Download UMat -> Mat for encoding (OpenCV imencode only accepts Mat)
        cv::Mat finalMat;
        blurred.copyTo(finalMat);

        // Prepare encode params
        std::vector<int> cvParams;
        for (int i = 0; i < encodeParamCount; i++) {
            cvParams.push_back(encodeParams[i]);
        }

        // Encode to bytes
        std::vector<uchar> buf;
        if (!cv::imencode(ext, finalMat, buf, cvParams)) {
            return result;
        }

        // Copy to malloc'd buffer (Go will read via C.GoBytes then free)
        result.data = (unsigned char*)malloc(buf.size());
        if (!result.data) return result;
        memcpy(result.data, buf.data(), buf.size());
        result.length = (int)buf.size();
        result.success = 0;
        return result;
    } catch (const std::exception& e) {
        return result;
    } catch (...) {
        return result;
    }
}

void GPU_FreeResultData(GPUCompressResult* result) {
    if (result && result->data) {
        free(result->data);
        result->data = NULL;
        result->length = 0;
    }
}

} // extern "C"
