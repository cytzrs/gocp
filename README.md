# gocp

# 例子

```

import cp "github.com/cytzrs/gocp"

buf, err := cp.Compress(p, "jpg", 25)
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