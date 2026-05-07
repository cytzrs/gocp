package cp

import (
	"image/color"

	"gocv.io/x/gocv"
)

// WatermarkConfig 文字水印配置。
// 注：gocv 内置 Hershey 字体，仅支持 ASCII 字符。
// 如需中文水印，Label 请使用英文或数字。
type WatermarkConfig struct {
	Labels    []string      // 多行文字，逐行显示在左下角
	FontFace  gocv.HersheyFont // 字体样式，默认 FontHersheyPlain
	FontScale float64       // 字体缩放（≈像素高度/30），默认 0.8
	Color     color.RGBA    // 文字颜色，默认白色 {255,255,255,0}
	Padding   int           // 距边缘内边距（像素），默认 20
}

type ImageCompressor struct {
	Quality  int
	Format   string
	Resize   bool
	Height   int
	Width    int
	Watermark *WatermarkConfig // 非 nil 时启用水印
}

type Option func(*ImageCompressor)

func NewImageCompressor(opts ...Option) *ImageCompressor {
	param := &ImageCompressor{}
	for _, f := range opts {
		f(param)
	}
	return param
}

func WithQuality(quality int) Option {
	return func(ic *ImageCompressor) {
		ic.Quality = quality
	}
}

func WithFormat(format string) Option {
	return func(ic *ImageCompressor) {
		ic.Format = format
	}
}

func WithResize(resize bool, width, height int) Option {
	return func(ic *ImageCompressor) {
		ic.Resize = resize
		ic.Width = width
		ic.Height = height
	}
}

// WithWatermark 启用水印。传入 nil 则不打水印。
func WithWatermark(wc *WatermarkConfig) Option {
	return func(ic *ImageCompressor) {
		ic.Watermark = wc
	}
}
