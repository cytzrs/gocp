# gocp

# 例子

```

import cp "github.com/cytzrs/gocp"

imgPath := "lion.jpg"
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

```