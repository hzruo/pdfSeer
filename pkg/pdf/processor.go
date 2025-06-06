package pdf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/h2non/bimg"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	imageprocessor "pdf-ocr-ai/pkg/image"
)

// PDFPage PDF页面信息
type PDFPage struct {
	Number      int     `json:"number"`
	Text        string  `json:"text"`         // PDF原生文本
	OCRText     string  `json:"ocr_text"`     // OCR识别文本
	AIText      string  `json:"ai_text"`      // AI处理后文本
	ImagePath   string  `json:"image_path"`   // 渲染图片路径
	HasText     bool    `json:"has_text"`     // 是否包含原生文本
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Processed   bool    `json:"processed"`    // 是否已处理
}

// PDFDocument PDF文档
type PDFDocument struct {
	FilePath    string     `json:"file_path"`
	Pages       []*PDFPage `json:"pages"`
	PageCount   int        `json:"page_count"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Subject     string     `json:"subject"`
	mu          sync.RWMutex
}

// PDFProcessor PDF处理器
type PDFProcessor struct {
	tempDir        string
	imageProcessor *imageprocessor.ImageProcessor
}

// NewPDFProcessor 创建PDF处理器
func NewPDFProcessor() (*PDFProcessor, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "pdf-ocr-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 创建图片处理器
	imageConfig := imageprocessor.ProcessorConfig{
		MaxWidth:    1600,
		MaxHeight:   2400,
		Quality:     90,
		Format:      "jpeg",
		Compression: true,
	}
	imageProcessor := imageprocessor.NewImageProcessor(imageConfig)

	return &PDFProcessor{
		tempDir:        tempDir,
		imageProcessor: imageProcessor,
	}, nil
}

// LoadPDF 加载PDF文件
func (p *PDFProcessor) LoadPDF(filePath string) (*PDFDocument, error) {
	// 获取页数
	pageCount, err := api.PageCountFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取页数失败: %w", err)
	}

	fmt.Printf("[DEBUG] PDF文件 %s 共有 %d 页\n", filePath, pageCount)

	doc := &PDFDocument{
		FilePath:  filePath,
		PageCount: pageCount,
		Pages:     make([]*PDFPage, 0, pageCount),
		Title:     filepath.Base(filePath),
		Author:    "",
		Subject:   "",
	}

	// 创建页面信息（不立即渲染，尺寸将在渲染时获取）
	for i := 1; i <= pageCount; i++ {
		page := &PDFPage{
			Number:  i,
			HasText: false,
			// 尺寸将在渲染时从 libvips 获取，暂时设为0
			Width:  0,
			Height: 0,
		}
		doc.Pages = append(doc.Pages, page)
	}

	return doc, nil
}

// GetPDFPath 获取 PDF 文件路径（用于浏览器预览）
func (p *PDFProcessor) GetPDFPath(doc *PDFDocument) string {
	return doc.FilePath
}

// RenderPageToImage 将页面渲染为图片
func (p *PDFProcessor) RenderPageToImage(doc *PDFDocument, pageNum int) (string, error) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return "", fmt.Errorf("页码超出范围: %d", pageNum)
	}

	// 检查是否已经渲染过
	page := doc.Pages[pageNum-1]
	if page.ImagePath != "" {
		if _, err := os.Stat(page.ImagePath); err == nil {
			fmt.Printf("[DEBUG] 第%d页已存在缓存图片: %s\n", pageNum, page.ImagePath)
			return page.ImagePath, nil
		}
	}

	fmt.Printf("[DEBUG] 开始渲染第%d页，PDF文件: %s\n", pageNum, doc.FilePath)

	var imagePath string
	var err error

	// 尝试使用 bimg 渲染 PDF 页面
	imagePath, err = p.renderWithBimg(doc.FilePath, pageNum, doc)
	if err != nil {
		fmt.Printf("[WARN] bimg 渲染失败: %v，尝试创建占位符\n", err)
		// 如果 bimg 渲染失败，创建占位符图片
		imagePath, err = p.createPlaceholderImageFile(pageNum, fmt.Sprintf("第%d页 - 渲染失败", pageNum))
		if err != nil {
			return "", fmt.Errorf("创建占位符图片失败: %w", err)
		}
		fmt.Printf("[DEBUG] 第%d页占位符图片创建成功\n", pageNum)
	} else {
		fmt.Printf("[DEBUG] 使用 bimg 渲染第%d页成功\n", pageNum)
	}

	// 更新页面信息
	doc.mu.Lock()
	doc.Pages[pageNum-1].ImagePath = imagePath
	doc.mu.Unlock()

	return imagePath, nil
}

// renderWithBimg 使用原生 libvips 渲染 PDF 页面
func (p *PDFProcessor) renderWithBimg(pdfPath string, pageNum int, doc *PDFDocument) (string, error) {
	fmt.Printf("[DEBUG] 使用原生 libvips 渲染第%d页，PDF文件: %s\n", pageNum, pdfPath)

	// 使用原生 libvips 渲染 PDF 页面
	result, err := p.renderPDFPageWithVips(pdfPath, pageNum)
	if err != nil {
		fmt.Printf("[WARN] 原生 libvips 渲染失败: %v，尝试使用 pdfcpu + bimg 方法\n", err)
		return p.renderWithBimgFallback(pdfPath, pageNum)
	}

	// 保存图片到文件
	imagePath := filepath.Join(p.tempDir, fmt.Sprintf("page_%d_vips.jpg", pageNum))
	err = ioutil.WriteFile(imagePath, result.ImageData, 0644)
	if err != nil {
		return "", fmt.Errorf("保存图片文件失败: %w", err)
	}

	// 更新页面尺寸信息
	if doc != nil && pageNum >= 1 && pageNum <= len(doc.Pages) {
		doc.mu.Lock()
		doc.Pages[pageNum-1].Width = float64(result.Width)
		doc.Pages[pageNum-1].Height = float64(result.Height)
		doc.mu.Unlock()
		fmt.Printf("[DEBUG] 更新第%d页尺寸信息: %dx%d\n", pageNum, result.Width, result.Height)
	}

	fmt.Printf("[DEBUG] 原生 libvips 渲染第%d页成功，输出文件: %s\n", pageNum, imagePath)
	return imagePath, nil
}

// renderWithBimgFallback 使用 pdfcpu + bimg 作为备用方案
func (p *PDFProcessor) renderWithBimgFallback(pdfPath string, pageNum int) (string, error) {
	fmt.Printf("[DEBUG] 使用 pdfcpu + bimg 备用方案渲染第%d页\n", pageNum)

	// 首先使用 pdfcpu 提取单页PDF
	singlePagePath := filepath.Join(p.tempDir, fmt.Sprintf("single_page_%d.pdf", pageNum))

	// 使用 pdfcpu 提取指定页面
	err := api.ExtractPagesFile(pdfPath, singlePagePath, []string{fmt.Sprintf("%d", pageNum)}, nil)
	if err != nil {
		return "", fmt.Errorf("提取第%d页失败: %w", pageNum, err)
	}

	fmt.Printf("[DEBUG] 成功提取第%d页到: %s\n", pageNum, singlePagePath)
	defer os.Remove(singlePagePath) // 清理临时文件

	// 读取单页PDF文件
	pdfData, err := ioutil.ReadFile(singlePagePath)
	if err != nil {
		return "", fmt.Errorf("读取单页PDF文件失败: %w", err)
	}

	fmt.Printf("[DEBUG] 单页PDF文件大小: %d bytes\n", len(pdfData))

	// 配置 bimg 选项
	options := bimg.Options{
		Type:    bimg.JPEG,
		Quality: 90,
		Width:   800,  // 设置宽度
		Height:  1000, // 设置高度
		Crop:    false,
		Enlarge: true,
	}

	// 尝试转换单页PDF为图片
	imageData, err := bimg.NewImage(pdfData).Process(options)
	if err != nil {
		return "", fmt.Errorf("bimg 处理失败: %w", err)
	}

	// 保存图片到文件
	imagePath := filepath.Join(p.tempDir, fmt.Sprintf("page_%d_bimg.jpg", pageNum))
	err = ioutil.WriteFile(imagePath, imageData, 0644)
	if err != nil {
		return "", fmt.Errorf("保存图片文件失败: %w", err)
	}

	fmt.Printf("[DEBUG] pdfcpu + bimg 渲染第%d页成功，输出文件: %s\n", pageNum, imagePath)
	return imagePath, nil
}

// GetPageImage 获取页面图片（如果不存在则渲染）
func (p *PDFProcessor) GetPageImage(doc *PDFDocument, pageNum int) ([]byte, error) {
	imagePath, err := p.RenderPageToImage(doc, pageNum)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(imagePath)
}

// UpdatePageOCR 更新页面OCR结果
func (p *PDFProcessor) UpdatePageOCR(doc *PDFDocument, pageNum int, ocrText string) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()
	
	doc.Pages[pageNum-1].OCRText = ocrText
	doc.Pages[pageNum-1].Processed = true
}

// UpdatePageAI 更新页面AI处理结果
func (p *PDFProcessor) UpdatePageAI(doc *PDFDocument, pageNum int, aiText string) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()
	
	doc.Pages[pageNum-1].AIText = aiText
}

// GetPage 获取页面信息
func (p *PDFProcessor) GetPage(doc *PDFDocument, pageNum int) *PDFPage {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return nil
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()
	
	return doc.Pages[pageNum-1]
}



// createPlaceholderImageFile 创建占位符图片文件
func (p *PDFProcessor) createPlaceholderImageFile(pageNum int, errorMsg string) (string, error) {
	// 创建一个有内容的占位符图片
	width, height := 800, 1000
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 填充浅灰色背景
	bgColor := color.RGBA{245, 245, 245, 255} // 浅灰色背景
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bgColor)
		}
	}

	// 添加边框
	borderColor := color.RGBA{180, 180, 180, 255} // 深灰色边框
	borderWidth := 3
	for i := 0; i < borderWidth; i++ {
		// 上下边框
		for x := 0; x < width; x++ {
			img.Set(x, i, borderColor)
			img.Set(x, height-1-i, borderColor)
		}
		// 左右边框
		for y := 0; y < height; y++ {
			img.Set(i, y, borderColor)
			img.Set(width-1-i, y, borderColor)
		}
	}

	// 添加页码信息（简单的像素绘制）
	p.drawSimpleText(img, fmt.Sprintf("Page %d", pageNum), width/2-40, height/2-20, color.RGBA{100, 100, 100, 255})

	// 添加一些装饰性的网格线
	gridColor := color.RGBA{220, 220, 220, 255}
	for x := 100; x < width-100; x += 50 {
		for y := 200; y < height-200; y += 2 {
			img.Set(x, y, gridColor)
		}
	}
	for y := 200; y < height-200; y += 50 {
		for x := 100; x < width-100; x += 2 {
			img.Set(x, y, gridColor)
		}
	}

	// 保存到文件
	imagePath := filepath.Join(p.tempDir, fmt.Sprintf("page_%d_placeholder.png", pageNum))
	file, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("创建占位符文件失败: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return "", fmt.Errorf("编码占位符图片失败: %w", err)
	}

	fmt.Printf("[DEBUG] 创建了 %dx%d 的占位符图片文件: %s\n", width, height, imagePath)
	return imagePath, nil
}

// drawSimpleText 简单的文字绘制（像素级别）
func (p *PDFProcessor) drawSimpleText(img *image.RGBA, text string, startX, startY int, textColor color.RGBA) {
	// 简单的文字绘制，只绘制一些基本的像素点来表示文字
	for i, char := range text {
		x := startX + i*8
		if x >= img.Bounds().Max.X-10 {
			break
		}

		// 为每个字符绘制一个简单的矩形
		for dx := 0; dx < 6; dx++ {
			for dy := 0; dy < 8; dy++ {
				if x+dx < img.Bounds().Max.X && startY+dy < img.Bounds().Max.Y &&
				   x+dx >= 0 && startY+dy >= 0 {
					// 根据字符绘制不同的模式
					if (dx == 0 || dx == 5 || dy == 0 || dy == 7) && char != ' ' {
						img.Set(x+dx, startY+dy, textColor)
					}
				}
			}
		}
	}
}

// drawText 简单的文字绘制（像素级别）
func (p *PDFProcessor) drawText(img *image.RGBA, text string, startX, startY int, textColor color.RGBA) {
	// 这是一个非常简单的文字绘制实现
	// 在实际项目中，您可能想要使用更专业的字体渲染库

	// 简单的 5x7 像素字体模式（仅支持数字和基本字符）
	patterns := map[rune][][]bool{
		'0': {
			{false, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
		},
		'1': {
			{false, false, true, false, false},
			{false, true, true, false, false},
			{false, false, true, false, false},
			{false, false, true, false, false},
			{false, false, true, false, false},
			{false, false, true, false, false},
			{false, true, true, true, false},
		},
		'2': {
			{false, true, true, true, false},
			{true, false, false, false, true},
			{false, false, false, false, true},
			{false, false, false, true, false},
			{false, false, true, false, false},
			{false, true, false, false, false},
			{true, true, true, true, true},
		},
		'3': {
			{false, true, true, true, false},
			{true, false, false, false, true},
			{false, false, false, false, true},
			{false, false, true, true, false},
			{false, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
		},
		'4': {
			{false, false, false, true, false},
			{false, false, true, true, false},
			{false, true, false, true, false},
			{true, false, false, true, false},
			{true, true, true, true, true},
			{false, false, false, true, false},
			{false, false, false, true, false},
		},
		'5': {
			{true, true, true, true, true},
			{true, false, false, false, false},
			{true, true, true, true, false},
			{false, false, false, false, true},
			{false, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
		},
		'6': {
			{false, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, false},
			{true, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
		},
		'7': {
			{true, true, true, true, true},
			{false, false, false, false, true},
			{false, false, false, true, false},
			{false, false, true, false, false},
			{false, true, false, false, false},
			{false, true, false, false, false},
			{false, true, false, false, false},
		},
		'8': {
			{false, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
		},
		'9': {
			{false, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, true},
			{false, false, false, false, true},
			{true, false, false, false, true},
			{false, true, true, true, false},
		},
		' ': {
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
			{false, false, false, false, false},
		},
	}

	x := startX
	for _, char := range text {
		if pattern, exists := patterns[char]; exists {
			for row, line := range pattern {
				for col, pixel := range line {
					if pixel && x+col >= 0 && x+col < img.Bounds().Max.X && startY+row >= 0 && startY+row < img.Bounds().Max.Y {
						img.Set(x+col, startY+row, textColor)
						// 加粗效果
						img.Set(x+col+1, startY+row, textColor)
						img.Set(x+col, startY+row+1, textColor)
						img.Set(x+col+1, startY+row+1, textColor)
					}
				}
			}
		}
		x += 8 // 字符间距
	}
}



// Cleanup 清理临时文件
func (p *PDFProcessor) Cleanup() error {
	return os.RemoveAll(p.tempDir)
}
