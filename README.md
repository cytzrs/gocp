# gocp

# 例子

```

import cp "github.com/cytzrs/gocp"

imgPath := "lion.jpg"
buf, err := cp.Compress(imgPath, "jpg", 25)   // 参数1: 图片路径 参数2:输出格式 参数3:输出质量
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