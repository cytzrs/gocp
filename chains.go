package cp

// import (
// 	"fmt"
// 	"image"

// 	"gocv.io/x/gocv"
// )

// type CompressorBuilder func(next Filter) Filter
// type Filter func(img gocv.Mat, p *ImageCompressor) (*gocv.Mat, error)

// func MakeK8sCMUpdateChain(request *ImageCompressor) (*gocv.Mat, error) {
// 	return MakeChain(request, []CompressorBuilder{ResizeBuilder})
// }

// func MakeChain(request *ImageCompressor, builders []CompressorBuilder) (*gocv.Mat, error) {
// 	chains := make([]CompressorBuilder, 0)
// 	chains = append(chains, builders...)

// 	img := gocv.IMRead(request.ImgPath, gocv.IMReadColor)
// 	defer img.Close()
// 	var root Filter = func(img gocv.Mat, p *ImageCompressor) (*gocv.Mat, error) {
// 		return nil, nil
// 	}

// 	for i := len(chains) - 1; i >= 0; i-- {
// 		c := chains[i]
// 		root = c(root)
// 	}

// 	return root(img, request)
// }

// func ResizeBuilder(img gocv.Mat, param *ImageCompressor) (gocv.Mat, error) {
// 	resized := gocv.NewMat()
// 	defer resized.Close()

// 	err := gocv.Resize(img, &resized, image.Point{min(img.Cols(), param.Width), min(img.Rows(), param.Height)}, 0, 0, gocv.InterpolationArea)

// 	return resized, err
// }

// func Switch2CM(next Filter) Filter {
// 	return func(img gocv.Mat, p *ImageCompressor) (*gocv.Mat, error) {
// 		resizer := func(worker *ImageCompressor) (*gocv.Mat, error) {
// 			return nil, nil
// 		}

// 		resource, err := resizer(worker)
// 		if err != nil {
// 			return fmt.Errorf("获取configmap失败: %s, params: %+v", err.Error(), *worker)
// 		}
// 		return next(resource, worker)
// 	}
// }
