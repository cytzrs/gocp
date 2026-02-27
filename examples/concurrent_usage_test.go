package cp_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	cp "github.com/cytzrs/gocp"
	"github.com/panjf2000/ants/v2"
	"gocv.io/x/gocv"
)

// ============================================
// 场景1: 批量图像处理（文件批量压缩）
// ============================================

// BatchCompressOptions 批量压缩配置
type BatchCompressOptions struct {
	InputDir      string        // 输入目录
	OutputDir     string        // 输出目录
	Concurrency   int           // 并发数（建议 5-10）
	Quality       int           // 压缩质量
	Format        string        // 输出格式
	ResizeEnabled bool          // 是否启用缩放
	MaxWidth      int           // 最大宽度
	MaxHeight     int           // 最大高度
	Timeout       time.Duration // 单个任务超时时间
}

// BatchCompressResult 批量压缩结果
type BatchCompressResult struct {
	TotalCount    int64         // 总文件数
	SuccessCount  int64         // 成功数
	FailureCount  int64         // 失败数
	SavedBytes    int64         // 节省的字节数
	Duration      time.Duration // 总耗时
	Errors        []error       // 错误列表
}

// BatchCompress 批量压缩图像
func BatchCompress(opts BatchCompressOptions) (*BatchCompressResult, error) {
	startTime := time.Now()

	// 1. 准备输出目录
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 2. 扫描输入文件
	files, err := scanImageFiles(opts.InputDir)
	if err != nil {
		return nil, fmt.Errorf("扫描文件失败: %w", err)
	}

	if len(files) == 0 {
		return nil, errors.New("未找到图像文件")
	}

	log.Printf("找到 %d 个图像文件", len(files))

	// 3. 创建协程池
	pool, err := ants.NewPool(
		opts.Concurrency,
		ants.WithExpiryDuration(opts.Timeout),
		ants.WithPreAlloc(false), // 动态扩容，节省初始内存
	)
	if err != nil {
		return nil, fmt.Errorf("创建协程池失败: %w", err)
	}
	defer pool.Release()

	// 4. 准备压缩参数
	params := cp.NewImageCompressor(
		cp.WithQuality(opts.Quality),
		cp.WithFormat(opts.Format),
		cp.WithResize(opts.ResizeEnabled, opts.MaxWidth, opts.MaxHeight),
	)

	// 5. 使用原子计数器统计结果（并发安全）
	var (
		totalCount   = int64(len(files))
		successCount atomic.Int64
		failureCount atomic.Int64
		savedBytes   atomic.Int64
		errorMutex   sync.Mutex
		resultErrors []error
	)

	// 6. 提交任务到协程池
	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		file := file // 避免闭包捕获问题

		err := pool.Submit(func() {
			defer wg.Done()

			// 处理单个文件
			saved, err := processSingleFile(file, opts.OutputDir, params)
			if err != nil {
				failureCount.Add(1)

				// 收集错误（加锁保护）
				errorMutex.Lock()
				resultErrors = append(resultErrors,
					fmt.Errorf("%s: %w", filepath.Base(file), err))
				errorMutex.Unlock()
				return
			}

			successCount.Add(1)
			savedBytes.Add(saved)
		})

		if err != nil {
			wg.Done()
			log.Printf("提交任务失败: %v", err)
			failureCount.Add(1)
		}
	}

	// 7. 等待所有任务完成
	wg.Wait()

	// 8. 返回结果
	result := &BatchCompressResult{
		TotalCount:   totalCount,
		SuccessCount: successCount.Load(),
		FailureCount: failureCount.Load(),
		SavedBytes:   savedBytes.Load(),
		Duration:     time.Since(startTime),
		Errors:       resultErrors,
	}

	return result, nil
}

// processSingleFile 处理单个文件
func processSingleFile(inputPath, outputDir string, params *cp.ImageCompressor) (int64, error) {
	// 获取原始文件大小
	originalSize, err := fileSize(inputPath)
	if err != nil {
		return 0, err
	}

	// 构造输出路径
	relPath, err := filepath.Rel(filepath.Dir(inputPath), inputPath)
	if err != nil {
		relPath = filepath.Base(inputPath)
	}
	outputPath := filepath.Join(outputDir, relPath)
	ext := filepath.Ext(outputPath)
	outputPath = outputPath[:len(outputPath)-len(ext)] + "." + params.Format

	// 确保输出目录存在
	outputDirPath := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDirPath, 0755); err != nil {
		return 0, err
	}

	// 压缩图像
	buf, err := cp.Compress(inputPath, params)
	if err != nil {
		return 0, fmt.Errorf("压缩失败: %w", err)
	}

	if buf == nil {
		return 0, errors.New("压缩结果为空")
	}

	// 写入文件
	if err := os.WriteFile(outputPath, buf, 0644); err != nil {
		return 0, fmt.Errorf("写入文件失败: %w", err)
	}

	// 计算节省的字节数
	outputSize := int64(len(buf))
	saved := originalSize - outputSize
	if saved < 0 {
		saved = 0 // 压缩后可能更大
	}

	log.Printf("✓ %s -> %s (%.2f%%)",
		filepath.Base(inputPath),
		filepath.Base(outputPath),
		float64(saved)/float64(originalSize)*100)

	return saved, nil
}

// ============================================
// 场景2: HTTP 图像处理服务（第三方 API 调用）
// ============================================

// ImageProcessingService 图像处理服务
type ImageProcessingService struct {
	// 配置参数
	compressQuality int
	compressFormat  string
	maxWidth        int
	maxHeight       int

	// 协程池（用于限制并发）
	compressPool *ants.Pool
	optimizePool *ants.Pool

	// 统计指标
	activeRequests atomic.Int64
	totalRequests  atomic.Int64
	failedRequests atomic.Int64
}

// NewImageProcessingService 创建图像处理服务
func NewImageProcessingService(concurrency int) *ImageProcessingService {
	compressPool, _ := ants.NewPool(
		concurrency,
		ants.WithExpiryDuration(30*time.Second),
		ants.WithPreAlloc(false),
	)

	optimizePool, _ := ants.NewPool(
		concurrency/2, // Optimize 耗时更长，减少并发
		ants.WithExpiryDuration(30*time.Second),
		ants.WithPreAlloc(false),
	)

	return &ImageProcessingService{
		compressQuality: 75,
		compressFormat:  "jpg",
		maxWidth:        1920,
		maxHeight:       1080,
		compressPool:    compressPool,
		optimizePool:    optimizePool,
	}
}

// Close 关闭服务，释放资源
func (s *ImageProcessingService) Close() {
	s.compressPool.Release()
	s.optimizePool.Release()
}

// ProcessImageRequest 图像处理请求
type ProcessImageRequest struct {
	ImageData     []byte  // 图像数据
	RemoveBg      bool    // 是否移除绿色背景
	Resize        bool    // 是否调整大小
	Quality       int     // 压缩质量
	Format        string  // 输出格式
}

// ProcessImageResponse 图像处理响应
type ProcessImageResponse struct {
	Data        []byte  // 处理后的图像数据
	OriginalSize int    // 原始大小
	OutputSize   int    // 输出大小
	ProcessingTime time.Duration // 处理时间
}

// ProcessImage 处理图像（HTTP API 可调用）
func (s *ImageProcessingService) ProcessImage(req ProcessImageRequest) (*ProcessImageResponse, error) {
	startTime := time.Now()

	// 统计活跃请求
	s.activeRequests.Add(1)
	defer s.activeRequests.Add(-1)
	s.totalRequests.Add(1)

	// 验证请求
	if len(req.ImageData) == 0 {
		s.failedRequests.Add(1)
		return nil, errors.New("图像数据为空")
	}

	// 设置默认值
	if req.Quality == 0 {
		req.Quality = s.compressQuality
	}
	if req.Format == "" {
		req.Format = s.compressFormat
	}

	// 使用 sync.Mutex 等待协程池可用（避免直接丢弃请求）
	// 这里为了简化，使用同步方式处理
	result, err := s.processImageSync(req)
	if err != nil {
		s.failedRequests.Add(1)
		return nil, err
	}

	result.ProcessingTime = time.Since(startTime)
	return result, nil
}

// processImageSync 同步处理图像
func (s *ImageProcessingService) processImageSync(req ProcessImageRequest) (*ProcessImageResponse, error) {
	originalSize := len(req.ImageData)

	// 1. 如果需要移除背景，先解码为 Mat
	var resultData []byte
	var err error

	if req.RemoveBg {
		resultData, err = s.processWithBgRemoval(req)
	} else {
		resultData, err = s.processCompressOnly(req)
	}

	if err != nil {
		return nil, err
	}

	return &ProcessImageResponse{
		Data:         resultData,
		OriginalSize: originalSize,
		OutputSize:   len(resultData),
	}, nil
}

// processWithBgRemoval 处理图像（移除背景）
func (s *ImageProcessingService) processWithBgRemoval(req ProcessImageRequest) ([]byte, error) {
	// ⚠️ 关键：使用 defer 确保 Mat 被关闭
	src, err := gocv.IMDecode(req.ImageData, gocv.IMReadColor)
	if err != nil {
		return nil, fmt.Errorf("解码图像失败: %w", err)
	}
	defer src.Close() // 必须关闭！

	if src.Empty() {
		return nil, errors.New("图像解码失败")
	}

	// 移除绿色背景
	optimized, err := cp.Optimize(src)
	if err != nil {
		return nil, fmt.Errorf("背景移除失败: %w", err)
	}
	defer optimized.Close() // 必须关闭！

	// 编码为字节流
	buf, err := gocv.IMEncode(gocv.FileExt(".jpg"), optimized)
	if err != nil {
		return nil, fmt.Errorf("编码失败: %w", err)
	}
	defer buf.Close()

	// 如果需要压缩，再次处理
	if req.Quality > 0 {
		resultBuf, err := cp.CompressByBytes(buf.GetBytes(), cp.NewImageCompressor(
			cp.WithQuality(req.Quality),
			cp.WithFormat(req.Format),
			cp.WithResize(req.Resize, s.maxWidth, s.maxHeight),
		))
		if err != nil {
			return nil, fmt.Errorf("压缩失败: %w", err)
		}
		return resultBuf, nil
	}

	return buf.GetBytes(), nil
}

// processCompressOnly 仅压缩
func (s *ImageProcessingService) processCompressOnly(req ProcessImageRequest) ([]byte, error) {
	params := cp.NewImageCompressor(
		cp.WithQuality(req.Quality),
		cp.WithFormat(req.Format),
		cp.WithResize(req.Resize, s.maxWidth, s.maxHeight),
	)

	return cp.CompressByBytes(req.ImageData, params)
}

// GetStats 获取服务统计信息
func (s *ImageProcessingService) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_requests":  s.activeRequests.Load(),
		"total_requests":   s.totalRequests.Load(),
		"failed_requests":  s.failedRequests.Load(),
		"compress_pool_cap": s.compressPool.Cap(),
		"compress_pool_running": s.compressPool.Running(),
		"optimize_pool_cap": s.optimizePool.Cap(),
		"optimize_pool_running": s.optimizePool.Running(),
	}
}

// ============================================
// 场景3: 流水线处理（多阶段处理）
// ============================================

// PipelineStage 流水线阶段
type PipelineStage func(gocv.Mat) (gocv.Mat, error)

// ImageProcessor 图像流水线处理器
type ImageProcessor struct {
	stages []PipelineStage
}

// NewImageProcessor 创建图像处理器
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		stages: make([]PipelineStage, 0),
	}
}

// AddStage 添加处理阶段
func (p *ImageProcessor) AddStage(stage PipelineStage) *ImageProcessor {
	p.stages = append(p.stages, stage)
	return p
}

// Process 执行流水线处理
func (p *ImageProcessor) Process(img gocv.Mat) (gocv.Mat, error) {
	var current gocv.Mat
	var err error

	// ⚠️ 资源管理：跟踪需要关闭的 Mat
	defer func() {
		// 如果有错误，确保中间结果被关闭
		if err != nil && !current.Empty() {
			current.Close()
		}
	}()

	current = img // 不关闭输入的 img（调用者负责）

	for i, stage := range p.stages {
		var next gocv.Mat
		next, err = stage(current)
		if err != nil {
			return gocv.Mat{}, fmt.Errorf("阶段 %d 失败: %w", i, err)
		}

		// 如果不是第一个阶段，关闭上一个 Mat
		if i > 0 {
			current.Close()
		}

		current = next
	}

	return current, nil
}

// ============================================
// 辅助函数
// ============================================

func scanImageFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if isImageFile(path) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func isImageFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
