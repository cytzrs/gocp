package cp

import (
	"errors"
	"image"
	"os"

	"gocv.io/x/gocv"
)

func Compress(imgPath string, params *ImageCompressor) ([]byte, error) {
	img := gocv.IMRead(imgPath, gocv.IMReadColor)
	defer img.Close()

	if img.Empty() {
		return nil, errors.New("failed to read image")
	}

	if params.Resize {
		resized := gocv.NewMat()
		defer resized.Close()

		err := resize(img, &resized, params.Width, params.Height)
		if err != nil {
			return nil, errors.New("failed to resize image")
		}

		img.Close()
		img = resized.Clone()
	}

	outBuf, err := encodeImage(img, params.Quality, params.Format)
	if err != nil {
		return nil, errors.New("failed to encode image")
	}

	return outBuf, nil
}

func fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func resize(img gocv.Mat, resized *gocv.Mat, width, height int) error {
	newWidth := min(img.Cols(), width)
	newHeight := min(img.Rows(), height)
	err := gocv.Resize(img, resized, image.Point{newWidth, newHeight}, 0, 0, gocv.InterpolationArea)

	return err
}

func encodeImage(img gocv.Mat, quality int, format string) ([]byte, error) {
	params := []int{
		gocv.IMWriteJpegQuality, quality,
		gocv.IMWriteJpegOptimize, 1,
		gocv.IMWriteJpegChromaQuality, quality,
	}

	ext := ".jpg"
	if format == "webp" {
		params = []int{gocv.IMWriteWebpQuality, quality}
		ext = ".webp"
	}

	dst := gocv.NewMat()
	defer dst.Close()

	ksize := image.Point{3, 3}
	sigmaX := 1.0
	sigmaY := 1.0
	borderType := gocv.BorderConstant

	err := gocv.GaussianBlur(img, &dst, ksize, sigmaX, sigmaY, borderType)
	if err != nil {
		return nil, err
	}

	buf, err := gocv.IMEncodeWithParams(gocv.FileExt(ext), dst, params)
	if err != nil {
		return nil, err
	}

	return buf.GetBytes(), nil
}

func Optimize(src gocv.Mat) (gocv.Mat, error) {
	hsv := gocv.NewMat()
	defer hsv.Close()
	gocv.CvtColor(src, &hsv, gocv.ColorBGRToHSV)

	lowerGreen := gocv.NewMatFromScalar(gocv.NewScalar(35, 43, 46, 0), gocv.MatTypeCV8UC3)
	upperGreen := gocv.NewMatFromScalar(gocv.NewScalar(77, 255, 255, 0), gocv.MatTypeCV8UC3)
	defer lowerGreen.Close()
	defer upperGreen.Close()

	mask := gocv.NewMat()
	defer mask.Close()
	gocv.InRange(hsv, lowerGreen, upperGreen, &mask)

	gocv.BitwiseNot(mask, &mask)

	dst := gocv.NewMatWithSize(src.Rows(), src.Cols(), src.Type())
	defer dst.Close()
	dst.SetTo(gocv.NewScalar(255, 255, 255, 0)) // 白色背景

	src.CopyToWithMask(&dst, mask)

	return dst, nil
}
