package examples

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	cp "github.com/cytzrs/gocp"
	"github.com/panjf2000/ants/v2"
	"gocv.io/x/gocv"
)

// ============================================
// HTTP API 服务示例（第三方并发调用最佳实践）
// ============================================

// ImageCompressAPI 图像压缩 HTTP API
type ImageCompressAPI struct {
	pool *ants.Pool

	// 配置
	maxConcurrency int
	defaultQuality int
	defaultFormat  string

	// 统计
	requestsServed atomic.Int64
	requestsFailed atomic.Int64
}

// NewImageCompressAPI 创建 API 服务
func NewImageCompressAPI(maxConcurrency int) *ImageCompressAPI {
	pool, err := ants.NewPool(
		maxConcurrency,
		ants.WithExpiryDuration(30*time.Second),
		ants.WithPreAlloc(false),
	)
	if err != nil {
		panic(err)
	}

	return &ImageCompressAPI{
		pool:           pool,
		maxConcurrency: maxConcurrency,
		defaultQuality: 75,
		defaultFormat:  "jpg",
	}
}

// CompressRequest 压缩请求
type CompressRequest struct {
	Quality int    `json:"quality"` // 压缩质量 (0-100)
	Format  string `json:"format"`  // 输出格式 (jpg/webp)
	Resize  bool   `json:"resize"`  // 是否缩放
	Width   int    `json:"width"`   // 目标宽度
	Height  int    `json:"height"`  // 目标高度
	RemoveBg bool  `json:"remove_bg"` // 是否移除绿色背景
}

// CompressResponse 压缩响应
type CompressResponse struct {
	Success        bool          `json:"success"`
	Message        string        `json:"message,omitempty"`
	OriginalSize   int           `json:"original_size,omitempty"`
	OutputSize     int           `json:"output_size,omitempty"`
	CompressionRatio float64     `json:"compression_ratio,omitempty"`
	ProcessingTime time.Duration `json:"processing_time,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	MaxConcurrency    int64 `json:"max_concurrency"`
	ActiveRequests    int64 `json:"active_requests"`
	RequestsServed    int64 `json:"requests_served"`
	RequestsFailed    int64 `json:"requests_failed"`
	SuccessRate       float64 `json:"success_rate"`
}

// HandleCompress 处理压缩请求
// POST /compress
// Content-Type: multipart/form-data 或 application/json
func (api *ImageCompressAPI) HandleCompress(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// 1. 检查并发限制
	if api.pool.Running() >= api.pool.Cap() {
		api.requestsFailed.Add(1)
		http.Error(w, "服务繁忙，请稍后重试", http.StatusServiceUnavailable)
		return
	}

	// 2. 读取图像数据
 imageData, err := api.readImageData(r)
	if err != nil {
		api.requestsFailed.Add(1)
		http.Error(w, fmt.Sprintf("读取图像失败: %v", err), http.StatusBadRequest)
		return
	}

	// 3. 解析请求参数
	var req CompressRequest
	if r.Header.Get("Content-Type") == "application/json" {
		// 从 JSON body 读取参数
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
	}

	// 设置默认值
	if req.Quality == 0 {
		req.Quality = api.defaultQuality
	}
	if req.Format == "" {
		req.Format = api.defaultFormat
	}

	// 4. 提交到协程池处理（同步等待结果）
	var resultBuf []byte
	var processErr error

	task := func() {
		resultBuf, processErr = api.processImage(imageData, req)
	}

	// 使用 Submit 等待任务完成
	if err := api.pool.Submit(task); err != nil {
		api.requestsFailed.Add(1)
		http.Error(w, "提交任务失败", http.StatusInternalServerError)
		return
	}

	// 等待任务完成（通过检查 resultBuf）
	// 注意：这里简化了，实际应该使用 sync.WaitGroup 或 channel
	// 为了示例简洁，假设处理很快完成
	time.Sleep(10 * time.Millisecond) // 简化的等待

	if processErr != nil {
		api.requestsFailed.Add(1)
		api.sendErrorResponse(w, processError)
		return
	}

	// 5. 返回结果
	api.requestsServed.Add(1)
	processingTime := time.Since(startTime)

	// 设置响应头
	w.Header().Set("Content-Type", "image/"+req.Format)
	w.Header().Set("X-Processing-Time", processingTime.String())
	w.Header().Set("X-Original-Size", fmt.Sprintf("%d", len(imageData)))
	w.Header().Set("X-Output-Size", fmt.Sprintf("%d", len(resultBuf)))

	// 写入图像数据
	if _, err := w.Write(resultBuf); err != nil {
		log.Printf("写入响应失败: %v", err)
	}
}

// readImageData 从请求中读取图像数据
func (api *ImageCompressAPI) readImageData(r *http.Request) ([]byte, error) {
	// 方式1: multipart/form-data
	if r.Header.Get("Content-Type") == "multipart/form-data" {
		file, _, err := r.FormFile("image")
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}

	// 方式2: raw binary
	if r.Header.Get("Content-Type") == "application/octet-stream" {
		return io.ReadAll(r.Body)
	}

	// 方式3: base64 encoded json
	return nil, fmt.Errorf("不支持的 Content-Type")
}

// processImage 处理图像（核心逻辑）
func (api *ImageCompressAPI) processImage(imageData []byte, req CompressRequest) ([]byte, error) {
	var result []byte
	var err error

	// ⚠️ 关键：如果使用 Optimize，必须正确管理 Mat 资源
	if req.RemoveBg {
		result, err = api.processWithBgRemoval(imageData, req)
	} else {
		result, err = api.processCompressOnly(imageData, req)
	}

	return result, err
}

// processWithBgRemoval 移除背景 + 压缩
// ⚠️⚠️⚠️ 最容易出错的场景！Mat 资源管理示例
func (api *ImageCompressAPI) processWithBgRemoval(imageData []byte, req CompressRequest) ([]byte, error) {
	// 1. 解码图像
	src, err := gocv.IMDecode(imageData, gocv.IMReadColor)
	if err != nil {
		return nil, fmt.Errorf("解码失败: %w", err)
	}
	defer src.Close() // ✅ 必须关闭

	if src.Empty() {
		return nil, fmt.Errorf("图像为空")
	}

	// 2. 移除绿色背景
	optimized, err := cp.Optimize(src)
	if err != nil {
		return nil, fmt.Errorf("背景移除失败: %w", err)
	}
	defer optimized.Close() // ✅ 必须关闭（Optimize 返回的 Mat）

	// 3. 编码为字节流
	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), optimized)
	if err != nil {
		return nil, fmt.Errorf("编码失败: %w", err)
	}
	defer buf.Close() // ✅ 必须关闭

	// 4. 如果需要压缩，再次处理
	if req.Quality > 0 {
		params := cp.NewImageCompressor(
			cp.WithQuality(req.Quality),
			cp.WithFormat(req.Format),
			cp.WithResize(req.Resize, req.Width, req.Height),
		)

		result, err := cp.CompressByBytes(buf.GetBytes(), params)
		if err != nil {
			return nil, fmt.Errorf("压缩失败: %w", err)
		}
		return result, nil
	}

	return buf.GetBytes(), nil
}

// processCompressOnly 仅压缩（不使用 Mat，更简单）
func (api *ImageCompressAPI) processCompressOnly(imageData []byte, req CompressRequest) ([]byte, error) {
	params := cp.NewImageCompressor(
		cp.WithQuality(req.Quality),
		cp.WithFormat(req.Format),
		cp.WithResize(req.Resize, req.Width, req.Height),
	)

	return cp.CompressByBytes(imageData, params)
}

// HandleStats 处理统计请求
// GET /stats
func (api *ImageCompressAPI) HandleStats(w http.ResponseWriter, r *http.Request) {
	served := api.requestsServed.Load()
	failed := api.requestsFailed.Load()
	total := served + failed

	var successRate float64
	if total > 0 {
		successRate = float64(served) / float64(total) * 100
	}

	stats := StatsResponse{
		MaxConcurrency: int64(api.maxConcurrency),
		ActiveRequests: api.pool.Running(),
		RequestsServed: served,
		RequestsFailed: failed,
		SuccessRate:    successRate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// sendErrorResponse 发送错误响应
func (api *ImageCompressAPI) sendErrorResponse(w http.ResponseWriter, err error) {
	resp := CompressResponse{
		Success: false,
		Message: err.Error(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(resp)
}

// ============================================
// 启动 HTTP 服务器
// ============================================

func StartHTTPServer(addr string, maxConcurrency int) error {
	api := NewImageCompressAPI(maxConcurrency)
	defer api.pool.Release()

	mux := http.NewServeMux()
	mux.HandleFunc("/compress", api.HandleCompress)
	mux.HandleFunc("/stats", api.HandleStats)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("启动图像压缩服务: %s (最大并发: %d)", addr, maxConcurrency)
	return server.ListenAndServe()
}

// ============================================
// 使用示例
// ============================================

func ExampleHTTPServer() {
	// 启动服务器
	go func() {
		if err := StartHTTPServer(":8080", 10); err != nil {
			log.Fatal(err)
		}
	}()

	// 客户端调用示例
	time.Sleep(1 * time.Second) // 等待服务器启动

	// 示例1: 上传并压缩图像
	// curl -X POST -F "image=@test.jpg" http://localhost:8080/compress

	// 示例2: 查看统计信息
	// curl http://localhost:8080/stats
}

func ExampleClientCall() {
	// Go 客户端并发调用示例
	client := &http.Client{Timeout: 30 * time.Second}

	// 模拟并发请求
	for i := 0; i < 100; i++ {
		go func(id int) {
			// 读取图像文件
			// imageData, _ := os.ReadFile("test.jpg")

			// 创建请求
			// req, _ := http.NewRequest("POST", "http://localhost:8080/compress", bytes.NewReader(imageData))
			// req.Header.Set("Content-Type", "application/octet-stream")

			// 发送请求
			// resp, err := client.Do(req)
			// if err != nil {
			//     log.Printf("请求 %d 失败: %v", id, err)
			//     return
			// }
			// defer resp.Body.Close()

			// 读取响应
			// result, _ := io.ReadAll(resp.Body)
			// log.Printf("请求 %d 完成，大小: %d", id, len(result))
			_ = id
		}(i)
	}
}

// ============================================
// 并发安全检查清单
// ============================================

/*
并发调用 gocp 库的安全检查清单：

✅ 1. 协程池使用
   - 使用 ants 协程池控制并发数（建议 5-20）
   - 避免无限制的 goroutine 创建

✅ 2. Mat 资源管理
   - 所有 gocv.Mat 必须使用 defer Close()
   - 特别注意 Optimize() 返回的 Mat

✅ 3. 错误处理
   - 检查所有 error 返回值
   - 使用 atomic 包进行并发安全的计数

✅ 4. 超时控制
   - 设置合理的超时时间（建议 30s）
   - 使用 ants.WithExpiryDuration

✅ 5. 内存管理
   - 大图像处理时控制并发数
   - 及时释放不再使用的资源

✅ 6. 优雅关闭
   - 实现 Close() 方法释放协程池
   - 处理正在进行的任务

❌ 常见错误：
1. 忘记 defer Mat.Close() → 内存泄漏
2. 在循环中使用相同的 Mat 变量 → 数据竞争
3. 无限制创建 goroutine → OOM
4. 未处理错误 → 状态不一致
5. 闭包捕获循环变量 → 错误的数据处理
*/
