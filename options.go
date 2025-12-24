package cp

type ImageCompressor struct {
	Quality int
	Format  string
	Resize  bool
	Height  int
	Width   int
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
