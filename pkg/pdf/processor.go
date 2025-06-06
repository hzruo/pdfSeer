package pdf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// UpdatePageText 更新页面原生文本
func (p *PDFProcessor) UpdatePageText(doc *PDFDocument, pageNum int, text string) {
	if pageNum < 1 || pageNum > len(doc.Pages) {
		return
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	doc.Pages[pageNum-1].Text = text
	doc.Pages[pageNum-1].HasText = text != ""
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



// ExtractNativeText 提取PDF页面的原生文本
func (p *PDFProcessor) ExtractNativeText(filePath string, pageNum int) (string, bool, error) {
	fmt.Printf("[DEBUG] 开始提取第%d页原生文本，PDF文件: %s\n", pageNum, filePath)

	// 创建临时目录用于提取PDF内容
	tempDir, err := os.MkdirTemp("", "pdf_content_extract_")
	if err != nil {
		fmt.Printf("[WARN] 创建临时目录失败: %v\n", err)
		return "", false, err
	}
	defer os.RemoveAll(tempDir)

	// 使用pdfcpu提取指定页面的内容
	err = api.ExtractContentFile(filePath, tempDir, []string{fmt.Sprintf("%d", pageNum)}, nil)
	if err != nil {
		fmt.Printf("[WARN] 提取第%d页PDF内容失败: %v\n", pageNum, err)
		return "", false, err
	}

	// 查找生成的内容文件
	files, err := filepath.Glob(filepath.Join(tempDir, "*.txt"))
	if err != nil {
		fmt.Printf("[WARN] 查找内容文件失败: %v\n", err)
		return "", false, err
	}

	if len(files) == 0 {
		fmt.Printf("[DEBUG] 第%d页没有生成内容文件\n", pageNum)
		return "", false, nil
	}

	// 读取并解析PDF内容文件
	var allText strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("[WARN] 读取内容文件失败: %v\n", err)
			continue
		}

		// 解析PDF语法内容，提取文本
		text := p.parsePDFContent(string(content))
		if text != "" {
			allText.WriteString(text)
			allText.WriteString("\n")
		}
	}

	// 清理提取的文本
	extractedText := p.cleanExtractedText(allText.String())

	// 判断是否有有效文本
	hasText := len(extractedText) > 10 && len(strings.TrimSpace(extractedText)) > 5

	if hasText {
		fmt.Printf("[DEBUG] 第%d页原生文本提取成功，文本长度: %d\n", pageNum, len(extractedText))
	} else {
		fmt.Printf("[DEBUG] 第%d页无有效原生文本\n", pageNum)
	}

	return extractedText, hasText, nil
}

// parsePDFContent 解析PDF内容，提取其中的文本
func (p *PDFProcessor) parsePDFContent(content string) string {
	var textSegments []string

	// 使用正则表达式匹配PDF文本操作符
	// Tj 操作符：显示文本字符串
	tjRegex := regexp.MustCompile(`\(([^)]*)\)\s*Tj`)
	tjMatches := tjRegex.FindAllStringSubmatch(content, -1)

	for _, match := range tjMatches {
		if len(match) > 1 {
			text := p.decodePDFString(match[1])
			if text != "" {
				textSegments = append(textSegments, text)
			}
		}
	}

	// TJ 操作符：显示文本数组
	tjArrayRegex := regexp.MustCompile(`\[([^\]]*)\]\s*TJ`)
	tjArrayMatches := tjArrayRegex.FindAllStringSubmatch(content, -1)

	for _, match := range tjArrayMatches {
		if len(match) > 1 {
			// 解析数组中的文本字符串和数字（间距调整）
			arrayContent := match[1]

			// 匹配文本字符串和数字
			elementRegex := regexp.MustCompile(`\(([^)]*)\)|(-?\d+(?:\.\d+)?)`)
			elementMatches := elementRegex.FindAllStringSubmatch(arrayContent, -1)

			var arrayText strings.Builder
			for _, elementMatch := range elementMatches {
				if len(elementMatch) > 1 {
					if elementMatch[1] != "" {
						// 这是文本字符串
						text := p.decodePDFString(elementMatch[1])
						arrayText.WriteString(text)
					} else if elementMatch[2] != "" {
						// 这是数字（间距调整）
						// 负数表示增加间距，正数表示减少间距
						// 根据数值大小决定是否添加空格
						if spacing := elementMatch[2]; spacing != "" {
							if strings.HasPrefix(spacing, "-") && len(spacing) > 2 {
								// 较大的负数通常表示单词间的空格
								arrayText.WriteString(" ")
							}
						}
					}
				}
			}

			if arrayText.Len() > 0 {
				textSegments = append(textSegments, arrayText.String())
			}
		}
	}

	// 查找其他可能的文本模式
	// 有些PDF可能使用不同的文本操作符
	otherTextRegex := regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*Tj`)
	otherMatches := otherTextRegex.FindAllStringSubmatch(content, -1)

	for _, match := range otherMatches {
		if len(match) > 1 {
			text := p.decodeHexString(match[1])
			if text != "" {
				textSegments = append(textSegments, text)
			}
		}
	}

	// 将所有文本段合并并进行智能排版优化
	return p.optimizeTextLayout(textSegments)
}

// optimizeTextLayout 优化文本排版
func (p *PDFProcessor) optimizeTextLayout(textSegments []string) string {
	if len(textSegments) == 0 {
		return ""
	}

	// 合并所有文本段
	fullText := strings.Join(textSegments, " ")

	// 1. 处理常见的PDF编码问题
	fullText = p.fixPDFEncoding(fullText)

	// 2. 修复单词拆分问题
	fullText = p.fixWordSplitting(fullText)

	// 3. 处理行尾连字符
	fullText = p.fixHyphenation(fullText)

	// 4. 规范化空白字符
	fullText = p.normalizeWhitespace(fullText)

	// 5. 修复句子结构
	fullText = p.fixSentenceStructure(fullText)

	return fullText
}

// fixPDFEncoding 修复PDF编码问题
func (p *PDFProcessor) fixPDFEncoding(text string) string {
	// 处理常见的PDF编码转义序列
	replacements := map[string]string{
		`\201`: "'",     // 左单引号
		`\202`: "'",     // 右单引号
		`\203`: `"`,     // 左双引号
		`\204`: `"`,     // 右双引号
		`\205`: "…",     // 省略号
		`\206`: "–",     // en dash
		`\207`: "—",     // em dash
		`\210`: "",      // 删除
		`\211`: "",      // 删除
		`\212`: "",      // 删除
		`\213`: "",      // 删除
		`\214`: "",      // 删除
		`\215`: "",      // 删除
		`\216`: "",      // 删除
		`\217`: "",      // 删除
		`\220`: "",      // 删除
		`\221`: "",      // 删除
		`\222`: "",      // 删除
		`\223`: "",      // 删除
		`\224`: "",      // 删除
		`\225`: "",      // 删除
		`\226`: "",      // 删除
		`\227`: "",      // 删除
		`\230`: "",      // 删除
		`\231`: "",      // 删除
		`\232`: "",      // 删除
		`\233`: "",      // 删除
		`\234`: "",      // 删除
		`\235`: "",      // 删除
		`\236`: "",      // 删除
		`\237`: "",      // 删除
		`\240`: " ",     // 不间断空格
	}

	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	return text
}

// fixWordSplitting 修复单词拆分问题
func (p *PDFProcessor) fixWordSplitting(text string) string {
	// 修复常见的单词拆分问题
	wordFixes := map[string]string{
		"J a vaScript":     "JavaScript",
		"T ypeScript":      "TypeScript",
		"H TML":           "HTML",
		"C SS":            "CSS",
		"A PI":            "API",
		"U RL":            "URL",
		"H TTP":           "HTTP",
		"H TTPS":          "HTTPS",
		"J SON":           "JSON",
		"X ML":            "XML",
		"S QL":            "SQL",
		"P DF":            "PDF",
		"U I":             "UI",
		"U X":             "UX",
		"I D":             "ID",
		"V ue":            "Vue",
		"R eact":          "React",
		"A ngular":        "Angular",
		"N ode":           "Node",
		"N PM":            "NPM",
		"G it":            "Git",
		"G itHub":         "GitHub",
		"V irtual D OM":   "Virtual DOM",
		"D OM":            "DOM",
		"fron tend":       "frontend",
		"back end":        "backend",
		"full stack":      "fullstack",
		"web a pp":        "web app",
		"a pplica tion":   "application",
		"a pplica tions":  "applications",
		"developmen t":    "development",
		"managemen t":     "management",
		"environmen t":    "environment",
		"componen t":      "component",
		"componen ts":     "components",
		"framew ork":      "framework",
		"librar y":        "library",
		"ser ver":         "server",
		"righ t":          "right",
		"straigh t":       "straight",
		"in teractive":    "interactive",
		"scra tch":        "scratch",
		"founda tion":     "foundation",
		"con ten t":       "content",
		"con ten ts":      "contents",
		"con ven tion":    "convention",
		"con ven tions":   "conventions",
		"typogra phical":  "typographical",
		"significan t":    "significant",
		"essen tial":      "essential",
		"straigh tfor ward": "straightforward",
		"in troduce":      "introduce",
		"alwa ys":         "always",
		"wa y":            "way",
		"a wa y":          "away",
		"doesn ":          "doesn't",
		"doesn\\'":        "doesn't",
		"I t":             "It",
		"Y ou":            "You",
		"H ence":          "Hence",
		"W eb":            "Web",
		"Cha pter":        "Chapter",
		"A":               "A",
		"an y":            "any",
		"ha ve":           "have",
		"righ t a wa y":   "right away",
	}

	for broken, fixed := range wordFixes {
		text = strings.ReplaceAll(text, broken, fixed)
	}

	// 使用正则表达式修复更通用的拆分模式
	// 修复单个字母后跟空格的情况（如 "a pple" -> "apple"）
	singleLetterRegex := regexp.MustCompile(`\b([a-z]) ([a-z]{2,})`)
	text = singleLetterRegex.ReplaceAllString(text, "$1$2")

	return text
}

// decodePDFString 解码PDF字符串
func (p *PDFProcessor) decodePDFString(pdfStr string) string {
	// 处理PDF字符串中的转义字符
	result := strings.ReplaceAll(pdfStr, `\n`, "\n")
	result = strings.ReplaceAll(result, `\r`, "\r")
	result = strings.ReplaceAll(result, `\t`, "\t")
	result = strings.ReplaceAll(result, `\b`, "\b")
	result = strings.ReplaceAll(result, `\f`, "\f")
	result = strings.ReplaceAll(result, `\\`, "\\")
	result = strings.ReplaceAll(result, `\(`, "(")
	result = strings.ReplaceAll(result, `\)`, ")")

	// 处理八进制转义序列
	octalRegex := regexp.MustCompile(`\\([0-7]{1,3})`)
	result = octalRegex.ReplaceAllStringFunc(result, func(match string) string {
		octalStr := match[1:] // 去掉反斜杠
		if len(octalStr) > 0 {
			// 简单处理：如果是可打印字符范围，直接转换
			// 这里可以根据需要实现更完整的八进制转换
			return match // 暂时保持原样
		}
		return match
	})

	return result
}

// fixHyphenation 处理行尾连字符
func (p *PDFProcessor) fixHyphenation(text string) string {
	// 处理行尾连字符（如 "develop-\nment" -> "development"）
	hyphenRegex := regexp.MustCompile(`([a-z])-\s*\n\s*([a-z])`)
	text = hyphenRegex.ReplaceAllString(text, "$1$2")

	// 处理其他连字符情况
	hyphenSpaceRegex := regexp.MustCompile(`([a-z])-\s+([a-z])`)
	text = hyphenSpaceRegex.ReplaceAllString(text, "$1$2")

	return text
}

// normalizeWhitespace 规范化空白字符
func (p *PDFProcessor) normalizeWhitespace(text string) string {
	// 将多个连续的空格替换为单个空格
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// 处理标点符号前的空格
	text = regexp.MustCompile(`\s+([,.!?;:])`).ReplaceAllString(text, "$1")

	// 处理标点符号后的空格
	text = regexp.MustCompile(`([,.!?;:])\s*`).ReplaceAllString(text, "$1 ")

	// 处理括号内的空格
	text = regexp.MustCompile(`\(\s+`).ReplaceAllString(text, "(")
	text = regexp.MustCompile(`\s+\)`).ReplaceAllString(text, ")")

	// 去除首尾空白
	text = strings.TrimSpace(text)

	return text
}

// fixSentenceStructure 修复句子结构
func (p *PDFProcessor) fixSentenceStructure(text string) string {
	// 确保句子开头大写
	sentenceRegex := regexp.MustCompile(`([.!?]\s+)([a-z])`)
	text = sentenceRegex.ReplaceAllStringFunc(text, func(match string) string {
		parts := sentenceRegex.FindStringSubmatch(match)
		if len(parts) >= 3 {
			return parts[1] + strings.ToUpper(parts[2])
		}
		return match
	})

	// 确保文本开头大写
	if len(text) > 0 {
		firstChar := string(text[0])
		if firstChar >= "a" && firstChar <= "z" {
			text = strings.ToUpper(firstChar) + text[1:]
		}
	}

	// 处理常见的句子分隔问题
	text = strings.ReplaceAll(text, " . ", ". ")
	text = strings.ReplaceAll(text, " , ", ", ")
	text = strings.ReplaceAll(text, " ; ", "; ")
	text = strings.ReplaceAll(text, " : ", ": ")
	text = strings.ReplaceAll(text, " ! ", "! ")
	text = strings.ReplaceAll(text, " ? ", "? ")

	return text
}

// decodeHexString 解码十六进制字符串
func (p *PDFProcessor) decodeHexString(hexStr string) string {
	// 简单的十六进制解码
	// 这里可以实现更完整的十六进制到文本的转换
	// 暂时返回空字符串，因为需要更复杂的编码处理
	return ""
}

// cleanExtractedText 清理提取的文本
func (p *PDFProcessor) cleanExtractedText(text string) string {
	// 去除首尾空白
	text = strings.TrimSpace(text)

	// 将多个连续的空格替换为单个空格
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// 将多个连续的空行替换为单个空行
	text = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(text, "\n\n")

	// 去除行首行尾的空白字符，但保留换行
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	return strings.Join(lines, "\n")
}

// ExtractAllNativeText 提取PDF所有页面的原生文本
func (p *PDFProcessor) ExtractAllNativeText(doc *PDFDocument) error {
	fmt.Printf("[DEBUG] 开始提取PDF所有页面的原生文本，共%d页\n", doc.PageCount)

	for i := 1; i <= doc.PageCount; i++ {
		text, hasText, err := p.ExtractNativeText(doc.FilePath, i)
		if err != nil {
			// 提取失败不影响其他页面，继续处理
			fmt.Printf("[WARN] 第%d页原生文本提取失败: %v\n", i, err)
			continue
		}

		// 更新页面信息
		doc.mu.Lock()
		if i <= len(doc.Pages) {
			doc.Pages[i-1].Text = text
			doc.Pages[i-1].HasText = hasText
		}
		doc.mu.Unlock()
	}

	fmt.Printf("[DEBUG] PDF原生文本提取完成\n")
	return nil
}

// Cleanup 清理临时文件
func (p *PDFProcessor) Cleanup() error {
	return os.RemoveAll(p.tempDir)
}
