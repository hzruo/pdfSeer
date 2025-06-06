package image

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// ProcessorConfig 图片处理配置
type ProcessorConfig struct {
	MaxWidth    int     // 最大宽度
	MaxHeight   int     // 最大高度
	Quality     int     // JPEG质量 (1-100)
	Format      string  // 输出格式 (jpeg, png)
	Compression bool    // 是否启用压缩
}

// DefaultConfig 默认配置
func DefaultConfig() ProcessorConfig {
	return ProcessorConfig{
		MaxWidth:    1600,
		MaxHeight:   2400,
		Quality:     85,
		Format:      "jpeg",
		Compression: true,
	}
}

// ImageProcessor 图片处理器
type ImageProcessor struct {
	config ProcessorConfig
}

// NewImageProcessor 创建图片处理器
func NewImageProcessor(config ProcessorConfig) *ImageProcessor {
	return &ImageProcessor{
		config: config,
	}
}

// ProcessImage 处理图片文件
func (p *ImageProcessor) ProcessImage(inputPath string, outputPath string) error {
	// 打开输入文件
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("打开输入文件失败: %w", err)
	}
	defer inputFile.Close()

	// 解码图片
	img, format, err := image.Decode(inputFile)
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}

	// 处理图片
	processedImg := p.processImageData(img)

	// 创建输出文件
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outputFile.Close()

	// 编码并保存
	return p.encodeImage(processedImg, outputFile, format)
}

// ProcessImageFromReader 从Reader处理图片
func (p *ImageProcessor) ProcessImageFromReader(reader io.Reader) ([]byte, error) {
	// 解码图片
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	// 处理图片
	processedImg := p.processImageData(img)

	// 编码到字节数组
	var buf bytes.Buffer
	if err := p.encodeImage(processedImg, &buf, format); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// processImageData 处理图片数据
func (p *ImageProcessor) processImageData(img image.Image) image.Image {
	if !p.config.Compression {
		return img
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 计算新尺寸
	newWidth, newHeight := p.calculateNewSize(width, height)

	// 如果尺寸没有变化，直接返回原图
	if newWidth == width && newHeight == height {
		return img
	}

	// 创建新图片
	newImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// 使用高质量缩放算法
	draw.CatmullRom.Scale(newImg, newImg.Bounds(), img, bounds, draw.Over, nil)

	return newImg
}

// calculateNewSize 计算新尺寸
func (p *ImageProcessor) calculateNewSize(width, height int) (int, int) {
	maxWidth := p.config.MaxWidth
	maxHeight := p.config.MaxHeight

	if width <= maxWidth && height <= maxHeight {
		return width, height
	}

	// 计算缩放比例
	scaleX := float64(maxWidth) / float64(width)
	scaleY := float64(maxHeight) / float64(height)

	// 选择较小的缩放比例以保持宽高比
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

	return newWidth, newHeight
}

// encodeImage 编码图片
func (p *ImageProcessor) encodeImage(img image.Image, writer io.Writer, originalFormat string) error {
	format := p.config.Format
	if format == "" {
		format = originalFormat
	}

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		options := &jpeg.Options{
			Quality: p.config.Quality,
		}
		return jpeg.Encode(writer, img, options)
	case "png":
		encoder := &png.Encoder{
			CompressionLevel: png.BestCompression,
		}
		return encoder.Encode(writer, img)
	default:
		return fmt.Errorf("不支持的图片格式: %s", format)
	}
}

// GetImageInfo 获取图片信息
func (p *ImageProcessor) GetImageInfo(imagePath string) (*ImageInfo, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 解码图片获取尺寸
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return nil, fmt.Errorf("解码图片配置失败: %w", err)
	}

	return &ImageInfo{
		Width:    config.Width,
		Height:   config.Height,
		Format:   format,
		Size:     stat.Size(),
		Filename: filepath.Base(imagePath),
	}, nil
}

// ImageInfo 图片信息
type ImageInfo struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
	Size     int64  `json:"size"`
	Filename string `json:"filename"`
}

// OptimizeForOCR 为OCR优化图片
func (p *ImageProcessor) OptimizeForOCR(inputPath string, outputPath string) error {
	// OCR优化配置
	ocrConfig := ProcessorConfig{
		MaxWidth:    2000,  // OCR需要较高分辨率
		MaxHeight:   3000,
		Quality:     95,    // 高质量保证文字清晰
		Format:      "png", // PNG无损压缩
		Compression: true,
	}

	// 临时更改配置
	originalConfig := p.config
	p.config = ocrConfig
	defer func() {
		p.config = originalConfig
	}()

	return p.ProcessImage(inputPath, outputPath)
}

// BatchProcess 批量处理图片
func (p *ImageProcessor) BatchProcess(inputPaths []string, outputDir string) error {
	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	for _, inputPath := range inputPaths {
		// 生成输出文件名
		filename := filepath.Base(inputPath)
		ext := filepath.Ext(filename)
		name := strings.TrimSuffix(filename, ext)
		
		var outputExt string
		if p.config.Format == "jpeg" || p.config.Format == "jpg" {
			outputExt = ".jpg"
		} else {
			outputExt = ".png"
		}
		
		outputPath := filepath.Join(outputDir, name+"_processed"+outputExt)

		// 处理图片
		if err := p.ProcessImage(inputPath, outputPath); err != nil {
			return fmt.Errorf("处理图片 %s 失败: %w", inputPath, err)
		}
	}

	return nil
}

// EstimateProcessedSize 估算处理后的文件大小
func (p *ImageProcessor) EstimateProcessedSize(imagePath string) (int64, error) {
	info, err := p.GetImageInfo(imagePath)
	if err != nil {
		return 0, err
	}

	// 计算新尺寸
	newWidth, newHeight := p.calculateNewSize(info.Width, info.Height)
	
	// 估算压缩后大小
	pixelCount := int64(newWidth * newHeight)
	
	var estimatedSize int64
	if p.config.Format == "jpeg" || p.config.Format == "jpg" {
		// JPEG压缩比估算
		compressionRatio := float64(p.config.Quality) / 100.0
		estimatedSize = int64(float64(pixelCount) * 3 * compressionRatio * 0.1) // 经验公式
	} else {
		// PNG压缩比估算
		estimatedSize = pixelCount * 3 / 2 // PNG通常能压缩到原始大小的一半
	}

	return estimatedSize, nil
}
