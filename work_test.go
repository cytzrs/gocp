package cp

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
)

func Test_Work(t *testing.T) {
	inPath := "../destination/"
	files, err := walkImages(inPath)
	if err != nil {
		fmt.Println("walk error:", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("no images found")
		return
	}

	pool, err := ants.NewPool(5, ants.WithExpiryDuration(10*time.Minute), ants.WithPreAlloc(true))
	if err != nil {
		panic(fmt.Sprintf("register resource pool error: %v", err))
	}

	// spawn workers
	for _, p := range files {
		rel, err := filepath.Rel(inPath, p)
		if err != nil {
			rel = filepath.Base(p)
		}

		outPath := "./out"
		outPath = filepath.Join(outPath, rel)
		outPath = outPath[:len(outPath)-len(filepath.Ext(outPath))] + ".jpg"

		pool.Submit(func() {
			if len(p) == 0 {
				fmt.Println("=====================p _failed=====================")
				return
			}
			buf, err := Compress(p, &ImageCompressor{
				Format:  "jpg",
				Quality: 75,
				Resize:  true,
				Height:  4096,
				Width:   3072,
			})

			if err != nil {
				fmt.Println("compressed_failed:" + err.Error())
				return
			}

			if buf == nil {
				fmt.Println("=====================buf_failed=====================")
				return
			}

			if err := os.WriteFile(outPath, buf, 0644); err != nil {
				fmt.Println("write_failed:" + err.Error())
				return
			}
		})
	}

}

func walkImages(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type().IsRegular() {
			if isImageExt(p) {
				files = append(files, p)
			}
		}

		return nil
	})
	return files, err
}

func isImageExt(name string) bool {
	ext := filepath.Ext(name)
	switch extLower := strings.ToLower(ext); extLower {
	case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".webp":
		return true
	default:
		return false
	}
}
