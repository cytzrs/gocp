# gocp

# 例子

```

buf, err := Compress(p, "jpg", 25)
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