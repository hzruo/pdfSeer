package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pdf-ocr-ai/pkg/cache"
	"pdf-ocr-ai/pkg/config"
	"pdf-ocr-ai/pkg/document"
	"pdf-ocr-ai/pkg/history"
	"pdf-ocr-ai/pkg/ocr"
	"pdf-ocr-ai/pkg/pdf"
	"pdf-ocr-ai/pkg/system"
)

// ProgressUpdate 进度更新
type ProgressUpdate struct {
	Total       int    `json:"total"`
	Processed   int    `json:"processed"`
	CurrentPage int    `json:"current_page"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// App struct
type App struct {
	ctx               context.Context
	configManager     *config.ConfigManager
	cacheManager      *cache.CacheManager
	historyManager    *history.HistoryManager
	pdfProcessor      *pdf.PDFProcessor
	documentProcessor *document.DocumentProcessor
	ocrClient         *ocr.OpenAIClient
	currentDoc        *pdf.PDFDocument
	mu                sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	fmt.Printf("[DEBUG] startup 方法被调用\n")
	a.ctx = ctx

	// 检查系统依赖
	fmt.Printf("[DEBUG] 检查系统依赖\n")
	sysInfo := system.CheckDependencies()
	dependencyReport := system.FormatDependencyReport(sysInfo)
	fmt.Printf("[INFO] 系统依赖检查结果:\n%s", dependencyReport)

	// 发送依赖检查结果到前端
	runtime.EventsEmit(ctx, "dependency-check", sysInfo)

	// 初始化各个组件
	if err := a.initializeComponents(); err != nil {
		fmt.Printf("[ERROR] 初始化组件失败: %v\n", err)
		log.Printf("初始化组件失败: %v", err)
		runtime.EventsEmit(ctx, "error", fmt.Sprintf("初始化失败: %v", err))
	} else {
		fmt.Printf("[DEBUG] 所有组件初始化成功\n")
	}
}

// initializeComponents 初始化组件
func (a *App) initializeComponents() error {
	var err error

	// 初始化配置管理器
	a.configManager, err = config.NewConfigManager()
	if err != nil {
		return fmt.Errorf("初始化配置管理器失败: %w", err)
	}

	// 初始化缓存管理器
	a.cacheManager, err = cache.NewCacheManager()
	if err != nil {
		return fmt.Errorf("初始化缓存管理器失败: %w", err)
	}

	// 初始化历史记录管理器
	a.historyManager, err = history.NewHistoryManager()
	if err != nil {
		return fmt.Errorf("初始化历史记录管理器失败: %w", err)
	}

	// 初始化PDF处理器
	a.pdfProcessor, err = pdf.NewPDFProcessor()
	if err != nil {
		return fmt.Errorf("初始化PDF处理器失败: %w", err)
	}

	// 初始化文档处理器
	fmt.Printf("[DEBUG] 开始初始化文档处理器\n")
	a.documentProcessor, err = document.NewDocumentProcessor()
	if err != nil {
		fmt.Printf("[ERROR] 初始化文档处理器失败: %v\n", err)
		return fmt.Errorf("初始化文档处理器失败: %w", err)
	}
	fmt.Printf("[DEBUG] 文档处理器初始化成功\n")

	// 初始化OCR客户端
	aiConfig := a.configManager.GetAIConfig()
	if aiConfig.APIKey != "" {
		a.ocrClient = ocr.NewOpenAIClient(aiConfig)
	}

	return nil
}

// shutdown 应用关闭时清理资源
func (a *App) shutdown(ctx context.Context) {
	if a.cacheManager != nil {
		a.cacheManager.Close()
	}
	if a.historyManager != nil {
		a.historyManager.Close()
	}
	if a.pdfProcessor != nil {
		a.pdfProcessor.Cleanup()
	}
	if a.documentProcessor != nil {
		a.documentProcessor.Cleanup()
	}
	if a.ocrClient != nil {
		a.ocrClient.Close()
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// SelectFile 选择文件对话框
func (a *App) SelectFile() (string, error) {
	options := runtime.OpenDialogOptions{
		Title: "选择PDF文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PDF文件",
				Pattern:     "*.pdf",
			},
		},
	}

	return runtime.OpenFileDialog(a.ctx, options)
}

// LoadDocument 加载文档文件（支持多种格式）
func (a *App) LoadDocument(filePath string) error {
	fmt.Printf("[DEBUG] 开始加载文档: %s\n", filePath)
	a.mu.Lock()
	defer a.mu.Unlock()

	// 首先检查 documentProcessor 是否已初始化
	fmt.Printf("[DEBUG] documentProcessor 是否为 nil: %v\n", a.documentProcessor == nil)
	if a.documentProcessor == nil {
		fmt.Printf("[ERROR] documentProcessor 未初始化\n")
		return fmt.Errorf("documentProcessor 未初始化")
	}

	// 检查文件格式是否支持
	fmt.Printf("[DEBUG] 检查文件格式支持性\n")
	if !a.documentProcessor.IsSupported(filePath) {
		fmt.Printf("[ERROR] 不支持的文件格式: %s\n", filePath)
		return fmt.Errorf("不支持的文件格式")
	}

	// 加载文档
	fmt.Printf("[DEBUG] 开始加载文档内容\n")

	doc, err := a.documentProcessor.LoadDocument(filePath)
	if err != nil {
		fmt.Printf("[ERROR] 加载文档失败: %v\n", err)
		return fmt.Errorf("加载文档失败: %w", err)
	}

	fmt.Printf("[DEBUG] 文档加载成功，页数: %d\n", doc.PageCount)

	a.currentDoc = doc

	// 生成文档ID并检查缓存
	documentID, err := a.cacheManager.GenerateDocumentID(filePath)
	if err != nil {
		log.Printf("生成文档ID失败: %v", err)
	} else {
		// 尝试从缓存加载
		if err := a.loadFromCache(documentID); err != nil {
			log.Printf("从缓存加载失败: %v", err)
		}
	}

	// 通知前端文档已加载
	runtime.EventsEmit(a.ctx, "document-loaded", map[string]interface{}{
		"document":    doc,
		"document_id": documentID,
	})

	return nil
}

// LoadPDF 加载PDF文件（保持向后兼容）
func (a *App) LoadPDF(filePath string) error {
	return a.LoadDocument(filePath)
}

// GetCurrentDocument 获取当前文档
func (a *App) GetCurrentDocument() *pdf.PDFDocument {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentDoc
}

// GetPDFPath 获取当前 PDF 文件路径（用于浏览器预览）
func (a *App) GetPDFPath() (string, error) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		return "", fmt.Errorf("没有加载的文档")
	}

	return a.pdfProcessor.GetPDFPath(doc), nil
}

// GetPageImage 获取页面图片
func (a *App) GetPageImage(pageNumber int) ([]byte, error) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		return nil, fmt.Errorf("未加载PDF文档")
	}

	return a.pdfProcessor.GetPageImage(doc, pageNumber)
}

// ProcessPages 处理选中的页面
func (a *App) ProcessPages(pageNumbers []int) {
	go a.processPagesBatch(pageNumbers, false)
}

// ProcessPagesForce 强制重新处理指定页面（跳过缓存）
func (a *App) ProcessPagesForce(pageNumbers []int) {
	go a.processPagesBatch(pageNumbers, true)
}

// CheckProcessedPages 检查哪些页面已经处理过
func (a *App) CheckProcessedPages(pageNumbers []int) map[string]interface{} {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	result := map[string]interface{}{
		"total_pages":     len(pageNumbers),
		"processed_pages": []int{},
		"unprocessed_pages": []int{},
		"processed_count": 0,
	}

	if doc == nil {
		return result
	}

	var processedPages []int
	var unprocessedPages []int

	for _, pageNum := range pageNumbers {
		if cached := a.checkPageCache(pageNum); cached != nil {
			processedPages = append(processedPages, pageNum)
		} else {
			unprocessedPages = append(unprocessedPages, pageNum)
		}
	}

	result["processed_pages"] = processedPages
	result["unprocessed_pages"] = unprocessedPages
	result["processed_count"] = len(processedPages)

	return result
}

// GetConfig 获取配置
func (a *App) GetConfig() config.AppConfig {
	return a.configManager.GetConfig()
}

// UpdateConfig 更新配置
func (a *App) UpdateConfig(cfg config.AppConfig) error {
	if err := a.configManager.UpdateConfig(cfg); err != nil {
		return err
	}

	// 更新OCR客户端配置
	if a.ocrClient != nil {
		a.ocrClient.UpdateConfig(cfg.AI)
	} else if cfg.AI.APIKey != "" {
		a.ocrClient = ocr.NewOpenAIClient(cfg.AI)
	}

	return nil
}

// GetHistoryRecords 获取历史记录
func (a *App) GetHistoryRecords(limit int) ([]*history.HistoryRecord, error) {
	return a.historyManager.GetRecentRecords(limit)
}

// GetHistoryPages 获取历史记录页面
func (a *App) GetHistoryPages(historyID int) ([]*history.HistoryPage, error) {
	return a.historyManager.GetRecordPages(historyID)
}

// GetDocumentHistoryPages 获取文档所有历史记录的页面数据
func (a *App) GetDocumentHistoryPages(documentPath string) ([]*history.HistoryPage, error) {
	return a.historyManager.GetDocumentPages(documentPath)
}

// DeleteHistoryRecord 删除历史记录及相关数据
func (a *App) DeleteHistoryRecord(historyID int) error {
	// 获取历史记录信息
	record, err := a.historyManager.GetRecord(historyID)
	if err != nil {
		return fmt.Errorf("获取历史记录失败: %w", err)
	}
	if record == nil {
		return fmt.Errorf("历史记录不存在")
	}

	log.Printf("开始删除历史记录 ID=%d, 文档=%s", historyID, record.DocumentPath)

	// 1. 删除历史记录数据库记录
	if err := a.historyManager.DeleteRecord(historyID); err != nil {
		return fmt.Errorf("删除历史记录失败: %w", err)
	}
	log.Printf("已删除历史记录数据库记录")

	// 2. 检查是否还有其他历史记录使用同一文档
	otherRecords, err := a.historyManager.GetRecordsByDocumentPath(record.DocumentPath)
	if err != nil {
		log.Printf("检查其他历史记录失败: %v", err)
	}

	// 如果没有其他历史记录使用该文档，清理缓存数据
	if len(otherRecords) == 0 {
		log.Printf("没有其他历史记录使用文档 %s，开始清理缓存", record.DocumentPath)

		// 生成文档ID用于缓存查找
		documentID, err := a.cacheManager.GenerateDocumentID(record.DocumentPath)
		if err != nil {
			log.Printf("生成文档ID失败: %v", err)
		} else {
			// 删除缓存数据
			if err := a.cacheManager.DeleteDocument(documentID); err != nil {
				log.Printf("删除缓存数据失败: %v", err)
			} else {
				log.Printf("已删除缓存数据")
			}
		}

		// 如果当前加载的文档是被删除的文档，保持文档加载但清理处理状态
		a.mu.Lock()
		if a.currentDoc != nil && a.currentDoc.FilePath == record.DocumentPath {
			log.Printf("保持文档加载状态，但清理页面处理数据")
			// 清理页面的处理状态，但保持文档结构
			for i := range a.currentDoc.Pages {
				a.currentDoc.Pages[i].OCRText = ""
				a.currentDoc.Pages[i].AIText = ""
				a.currentDoc.Pages[i].Processed = false
			}
		}
		a.mu.Unlock()
	} else {
		log.Printf("文档 %s 还有 %d 个其他历史记录，保留缓存数据", record.DocumentPath, len(otherRecords))
	}

	log.Printf("历史记录删除完成")
	return nil
}

// SearchHistory 搜索历史记录
func (a *App) SearchHistory(keyword string, limit int) ([]*history.SearchResult, error) {
	return a.historyManager.SearchContent(keyword, limit)
}

// processPagesBatch 批量处理页面
func (a *App) processPagesBatch(pageNumbers []int, forceReprocess bool) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		log.Printf("未加载PDF文档，建议用户重新选择文件")
		runtime.EventsEmit(a.ctx, "processing-error", map[string]interface{}{
			"error":   "未加载PDF文档",
			"message": "请重新选择PDF文件。如果刚刚删除了历史记录，文档可能需要重新加载。",
			"code":    "DOCUMENT_NOT_LOADED",
			"action":  "RELOAD_DOCUMENT",
		})
		return
	}

	if a.ocrClient == nil {
		runtime.EventsEmit(a.ctx, "processing-error", "未配置AI服务")
		return
	}

	// 创建历史记录
	aiConfig := a.configManager.GetAIConfig()
	historyRecord, err := a.historyManager.CreateRecord(doc.FilePath, len(pageNumbers), aiConfig.Model)
	if err != nil {
		log.Printf("创建历史记录失败: %v", err)
	}

	// 发送初始进度
	runtime.EventsEmit(a.ctx, "processing-progress", ProgressUpdate{
		Total:     len(pageNumbers),
		Processed: 0,
		Status:    "开始处理",
	})

	// 使用并发处理
	processed := a.processPagesConcurrently(pageNumbers, historyRecord, doc, forceReprocess)

	// 更新历史记录状态
	if historyRecord != nil {
		a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCompleted, "")
	}

	// 发送完成通知
	runtime.EventsEmit(a.ctx, "processing-complete", map[string]interface{}{
		"total_processed": processed,
		"document":        doc,
	})
}

// processSinglePage 处理单个页面
func (a *App) processSinglePage(pageNum int, historyRecord *history.HistoryRecord) error {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		return fmt.Errorf("未加载PDF文档")
	}

	startTime := time.Now()

	// 渲染页面为图片
	imagePath, err := a.pdfProcessor.RenderPageToImage(doc, pageNum)
	if err != nil {
		return fmt.Errorf("渲染页面失败: %w", err)
	}

	// 使用AI识别文字（带重试机制）
	log.Printf("开始OCR识别页面 %d", pageNum)
	result, err := a.ocrClient.RecognizeImage(context.Background(), imagePath)
	if err != nil {
		log.Printf("页面 %d OCR识别失败: %v", pageNum, err)
		return fmt.Errorf("OCR识别失败: %w", err)
	}
	log.Printf("页面 %d OCR识别成功", pageNum)

	if result.Error != "" {
		return fmt.Errorf("OCR识别错误: %s", result.Error)
	}

	// 更新页面OCR结果
	a.pdfProcessor.UpdatePageOCR(doc, pageNum, result.Text)

	// 保存到缓存
	if err := a.savePageToCache(pageNum, result.Text, ""); err != nil {
		log.Printf("保存缓存失败: %v", err)
	}

	// 保存到历史记录
	if historyRecord != nil {
		page := &history.HistoryPage{
			HistoryID:      historyRecord.ID,
			PageNumber:     pageNum,
			OriginalText:   doc.Pages[pageNum-1].Text,
			OCRText:        result.Text,
			ProcessingTime: time.Since(startTime).Seconds(),
		}
		if err := a.historyManager.AddPage(page); err != nil {
			log.Printf("保存历史记录失败: %v", err)
		}
	}

	return nil
}

// loadFromCache 从缓存加载文档
func (a *App) loadFromCache(documentID string) error {
	if a.currentDoc == nil {
		return fmt.Errorf("当前文档为空")
	}

	// 获取缓存的页面
	pages, err := a.cacheManager.GetDocumentPages(documentID)
	if err != nil {
		return fmt.Errorf("获取缓存页面失败: %w", err)
	}

	// 更新文档页面信息
	for _, cachedPage := range pages {
		if cachedPage.PageNumber > 0 && cachedPage.PageNumber <= len(a.currentDoc.Pages) {
			page := a.currentDoc.Pages[cachedPage.PageNumber-1]
			if cachedPage.OCRText != "" {
				page.OCRText = cachedPage.OCRText
				page.Processed = true
			}
			if cachedPage.AIText != "" {
				page.AIText = cachedPage.AIText
			}
			if cachedPage.OriginalText != "" {
				page.Text = cachedPage.OriginalText
				page.HasText = true
			}
		}
	}

	return nil
}

// checkPageCache 检查页面缓存
func (a *App) checkPageCache(pageNum int) *cache.CacheEntry {
	if a.currentDoc == nil {
		return nil
	}

	documentID, err := a.cacheManager.GenerateDocumentID(a.currentDoc.FilePath)
	if err != nil {
		return nil
	}

	cached, err := a.cacheManager.GetPage(documentID, pageNum)
	if err != nil {
		return nil
	}

	return cached
}

// savePageToCache 保存页面到缓存
func (a *App) savePageToCache(pageNum int, ocrText, aiText string) error {
	if a.currentDoc == nil {
		return fmt.Errorf("当前文档为空")
	}

	documentID, err := a.cacheManager.GenerateDocumentID(a.currentDoc.FilePath)
	if err != nil {
		return fmt.Errorf("生成文档ID失败: %w", err)
	}

	// 保存文档信息
	docCache := &cache.DocumentCache{
		ID:        documentID,
		FilePath:  a.currentDoc.FilePath,
		PageCount: a.currentDoc.PageCount,
		Title:     a.currentDoc.Title,
		Author:    a.currentDoc.Author,
	}
	if err := a.cacheManager.SaveDocument(docCache); err != nil {
		log.Printf("保存文档缓存失败: %v", err)
	}

	// 保存页面信息
	var originalText string
	if pageNum > 0 && pageNum <= len(a.currentDoc.Pages) {
		originalText = a.currentDoc.Pages[pageNum-1].Text
	}

	pageCache := &cache.CacheEntry{
		DocumentID:   documentID,
		PageNumber:   pageNum,
		OriginalText: originalText,
		OCRText:      ocrText,
		AIText:       aiText,
	}

	return a.cacheManager.SavePage(pageCache)
}

// ProcessWithAI 使用AI处理文本
func (a *App) ProcessWithAI(pageNumbers []int, prompt string) {
	go a.processWithAI(pageNumbers, prompt)
}

// processWithAI AI处理文本
func (a *App) processWithAI(pageNumbers []int, prompt string) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		runtime.EventsEmit(a.ctx, "ai-processing-error", "未加载PDF文档")
		return
	}

	if a.ocrClient == nil {
		runtime.EventsEmit(a.ctx, "ai-processing-error", "未配置AI服务")
		return
	}

	// 收集文本
	var textBuilder strings.Builder
	for _, pageNum := range pageNumbers {
		if pageNum < 1 || pageNum > len(doc.Pages) {
			continue
		}
		page := doc.Pages[pageNum-1]
		text := page.OCRText
		if text == "" {
			text = page.Text
		}
		if text != "" {
			textBuilder.WriteString(fmt.Sprintf("=== 第 %d 页 ===\n%s\n\n", pageNum, text))
		}
	}

	if textBuilder.Len() == 0 {
		runtime.EventsEmit(a.ctx, "ai-processing-error", "没有可处理的文本")
		return
	}

	// 使用AI处理
	result, err := a.ocrClient.ProcessWithAI(context.Background(), textBuilder.String(), prompt)
	if err != nil {
		runtime.EventsEmit(a.ctx, "ai-processing-error", fmt.Sprintf("AI处理失败: %v", err))
		return
	}

	// 更新页面AI处理结果并保存到缓存
	for _, pageNum := range pageNumbers {
		if pageNum < 1 || pageNum > len(doc.Pages) {
			continue
		}

		// 更新页面AI处理结果
		a.pdfProcessor.UpdatePageAI(doc, pageNum, result)

		// 保存到缓存（保持现有的OCR文本，只更新AI文本）
		page := doc.Pages[pageNum-1]
		if err := a.savePageToCache(pageNum, page.OCRText, result); err != nil {
			log.Printf("保存AI处理结果到缓存失败: %v", err)
		}
	}

	// 发送结果
	runtime.EventsEmit(a.ctx, "ai-processing-complete", map[string]interface{}{
		"pages":  pageNumbers,
		"prompt": prompt,
		"result": result,
	})
}

// ExportText 导出文本
func (a *App) ExportText(pageNumbers []int, format string) (string, error) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		return "", fmt.Errorf("未加载PDF文档")
	}

	var builder strings.Builder

	for _, pageNum := range pageNumbers {
		if pageNum < 1 || pageNum > len(doc.Pages) {
			continue
		}

		page := doc.Pages[pageNum-1]
		text := page.OCRText
		if text == "" {
			text = page.Text
		}
		if text == "" {
			text = page.AIText
		}

		if text != "" {
			switch format {
			case "markdown":
				builder.WriteString(fmt.Sprintf("## 第 %d 页\n\n%s\n\n", pageNum, text))
			case "html":
				builder.WriteString(fmt.Sprintf("<h2>第 %d 页</h2>\n<p>%s</p>\n\n", pageNum, text))
			default: // txt
				builder.WriteString(fmt.Sprintf("=== 第 %d 页 ===\n%s\n\n", pageNum, text))
			}
		}
	}

	return builder.String(), nil
}

// ExportProcessingResults 导出批量处理结果
func (a *App) ExportProcessingResults(format string) (string, error) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		return "", fmt.Errorf("未加载PDF文档")
	}

	var builder strings.Builder
	processedCount := 0

	// 添加文档信息头部
	switch format {
	case "markdown":
		builder.WriteString(fmt.Sprintf("# %s - 处理结果\n\n", doc.Title))
		builder.WriteString(fmt.Sprintf("**文件路径:** %s\n\n", doc.FilePath))
		builder.WriteString(fmt.Sprintf("**总页数:** %d\n\n", doc.PageCount))
		builder.WriteString("---\n\n")
	case "html":
		builder.WriteString(fmt.Sprintf("<h1>%s - 处理结果</h1>\n", doc.Title))
		builder.WriteString(fmt.Sprintf("<p><strong>文件路径:</strong> %s</p>\n", doc.FilePath))
		builder.WriteString(fmt.Sprintf("<p><strong>总页数:</strong> %d</p>\n", doc.PageCount))
		builder.WriteString("<hr>\n")
	case "rtf":
		builder.WriteString("{\\rtf1\\ansi\\ansicpg936\\deff0\\deflang2052\n")
		builder.WriteString("{\\fonttbl{\\f0\\fswiss\\fcharset134 Microsoft YaHei;}{\\f1\\fmodern\\fcharset0 Courier New;}}\n")
		builder.WriteString("{\\colortbl;\\red0\\green0\\blue0;\\red0\\green0\\blue255;}\n")
		builder.WriteString(fmt.Sprintf("\\viewkind4\\uc1\\pard\\cf1\\lang2052\\f0\\fs28\\b %s - 处理结果\\par\n", doc.Title))
		builder.WriteString("\\par\n")
		builder.WriteString(fmt.Sprintf("\\cf0\\fs22\\b0\\f1 文件路径: %s\\par\n", doc.FilePath))
		builder.WriteString(fmt.Sprintf("总页数: %d\\par\n", doc.PageCount))
		builder.WriteString("\\par\n")
	default: // txt
		builder.WriteString(fmt.Sprintf("%s - 处理结果\n", doc.Title))
		builder.WriteString(fmt.Sprintf("文件路径: %s\n", doc.FilePath))
		builder.WriteString(fmt.Sprintf("总页数: %d\n", doc.PageCount))
		builder.WriteString("=" + strings.Repeat("=", 50) + "\n\n")
	}

	// 导出所有已处理的页面
	for i, page := range doc.Pages {
		if !page.Processed {
			continue
		}

		pageNum := i + 1
		processedCount++

		// 优先使用 OCR 结果，其次是 AI 结果，最后是原生文本
		text := page.OCRText
		if text == "" && page.AIText != "" {
			text = page.AIText
		}
		if text == "" && page.Text != "" {
			text = page.Text
		}

		if text != "" {
			switch format {
			case "markdown":
				builder.WriteString(fmt.Sprintf("## 第 %d 页\n\n", pageNum))
				builder.WriteString(fmt.Sprintf("%s\n\n", text))
			case "html":
				builder.WriteString(fmt.Sprintf("<h2>第 %d 页</h2>\n", pageNum))
				builder.WriteString(fmt.Sprintf("<div class=\"page-content\">%s</div>\n\n",
					strings.ReplaceAll(text, "\n", "<br>\n")))
			case "rtf":
				builder.WriteString(fmt.Sprintf("\\par\\b 第 %d 页\\b0\\par\\par", pageNum))
				// 转义RTF特殊字符
				rtfText := strings.ReplaceAll(text, "\\", "\\\\")
				rtfText = strings.ReplaceAll(rtfText, "{", "\\{")
				rtfText = strings.ReplaceAll(rtfText, "}", "\\}")
				rtfText = strings.ReplaceAll(rtfText, "\n", "\\par\n")
				builder.WriteString(fmt.Sprintf("%s\\par\\par", rtfText))
			default: // txt
				builder.WriteString(fmt.Sprintf("=== 第 %d 页 ===\n", pageNum))
				builder.WriteString(fmt.Sprintf("%s\n\n", text))
			}
		}
	}

	// 添加统计信息
	switch format {
	case "markdown":
		builder.WriteString("---\n\n")
		builder.WriteString(fmt.Sprintf("**处理统计:** 共处理 %d 页，总计 %d 页\n", processedCount, doc.PageCount))
	case "html":
		builder.WriteString("<hr>\n")
		builder.WriteString(fmt.Sprintf("<p><strong>处理统计:</strong> 共处理 %d 页，总计 %d 页</p>\n", processedCount, doc.PageCount))
	case "rtf":
		builder.WriteString("\\par\\par")
		builder.WriteString(fmt.Sprintf("\\b 处理统计:\\b0 共处理 %d 页，总计 %d 页\\par", processedCount, doc.PageCount))
		builder.WriteString("}")
	default: // txt
		builder.WriteString(strings.Repeat("=", 50) + "\n")
		builder.WriteString(fmt.Sprintf("处理统计: 共处理 %d 页，总计 %d 页\n", processedCount, doc.PageCount))
	}

	if processedCount == 0 {
		return "", fmt.Errorf("没有已处理的页面可以导出")
	}

	return builder.String(), nil
}

// SaveFileWithDialog 显示保存文件对话框并保存内容
func (a *App) SaveFileWithDialog(content string, defaultFilename string, filters []runtime.FileFilter) (string, error) {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Filters:         filters,
		Title:           "保存文件",
	})

	if err != nil {
		return "", err
	}

	if filePath == "" {
		// 用户取消了保存
		return "", nil
	}

	// 保存文件
	err = ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}

	return filePath, nil
}

// SaveBinaryFileWithDialog 显示保存文件对话框并保存二进制内容（base64编码）
func (a *App) SaveBinaryFileWithDialog(base64Content string, defaultFilename string, filters []runtime.FileFilter) (string, error) {
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Filters:         filters,
		Title:           "保存文件",
	})

	if err != nil {
		return "", err
	}

	if filePath == "" {
		// 用户取消了保存
		return "", nil
	}

	// 解码base64内容
	data, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return "", fmt.Errorf("解码base64内容失败: %w", err)
	}

	// 保存二进制文件
	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}

	return filePath, nil
}

// UpdatePageText 更新页面文本（用于编辑功能）
func (a *App) UpdatePageText(pageNumber int, textType string, text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.currentDoc == nil {
		return fmt.Errorf("未加载PDF文档")
	}

	if pageNumber < 1 || pageNumber > len(a.currentDoc.Pages) {
		return fmt.Errorf("页码超出范围")
	}

	page := a.currentDoc.Pages[pageNumber-1]

	switch textType {
	case "ocr":
		page.OCRText = text
	case "ai":
		page.AIText = text
	default:
		return fmt.Errorf("不支持的文本类型: %s", textType)
	}

	// 更新缓存
	var ocrText, aiText string
	if textType == "ocr" {
		ocrText = text
		aiText = page.AIText
	} else {
		ocrText = page.OCRText
		aiText = text
	}

	if err := a.savePageToCache(pageNumber, ocrText, aiText); err != nil {
		log.Printf("更新缓存失败: %v", err)
	}

	return nil
}

// ExtractNativeText 按需提取页面原生文本
func (a *App) ExtractNativeText(pageNumber int) (string, error) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		return "", fmt.Errorf("未加载PDF文档")
	}

	if pageNumber < 1 || pageNumber > len(doc.Pages) {
		return "", fmt.Errorf("页码超出范围")
	}

	page := doc.Pages[pageNumber-1]

	// 如果已经提取过，直接返回
	if page.Text != "" {
		return page.Text, nil
	}

	// 使用PDF处理器提取原生文本
	text, hasText, err := a.pdfProcessor.ExtractNativeText(doc.FilePath, pageNumber)
	if err != nil {
		return "", fmt.Errorf("提取原生文本失败: %w", err)
	}

	// 更新页面信息
	a.mu.Lock()
	page.Text = text
	page.HasText = hasText
	a.mu.Unlock()

	// 更新缓存
	if err := a.savePageToCache(pageNumber, page.OCRText, page.AIText); err != nil {
		log.Printf("更新缓存失败: %v", err)
	}

	return text, nil
}

// processPagesConcurrently 并发处理页面
func (a *App) processPagesConcurrently(pageNumbers []int, historyRecord *history.HistoryRecord, doc *pdf.PDFDocument, forceReprocess bool) int {
	const maxConcurrency = 3 // 限制并发数以避免API限制

	// 创建工作通道
	pagesChan := make(chan int, len(pageNumbers))
	resultsChan := make(chan ProcessResult, len(pageNumbers))

	// 发送页面到通道
	for _, pageNum := range pageNumbers {
		pagesChan <- pageNum
	}
	close(pagesChan)

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pageNum := range pagesChan {
				result := a.processPageWithResult(pageNum, historyRecord, doc, forceReprocess)
				resultsChan <- result
			}
		}()
	}

	// 等待所有工作完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果并发送进度更新
	processed := 0
	total := len(pageNumbers)

	for result := range resultsChan {
		processed++

		if result.Error != nil {
			log.Printf("处理第%d页失败: %v", result.PageNumber, result.Error)
			runtime.EventsEmit(a.ctx, "processing-error", fmt.Sprintf("处理第%d页失败: %v", result.PageNumber, result.Error))
		}

		runtime.EventsEmit(a.ctx, "processing-progress", ProgressUpdate{
			Total:       total,
			Processed:   processed,
			CurrentPage: result.PageNumber,
			Status:      result.Status,
		})
	}

	return processed
}

// ProcessResult 处理结果
type ProcessResult struct {
	PageNumber int
	Status     string
	Error      error
}

// processPageWithResult 处理页面并返回结果
func (a *App) processPageWithResult(pageNum int, historyRecord *history.HistoryRecord, doc *pdf.PDFDocument, forceReprocess bool) ProcessResult {
	startTime := time.Now()

	// 检查缓存（除非强制重新处理）
	if !forceReprocess {
		if cached := a.checkPageCache(pageNum); cached != nil {
			a.pdfProcessor.UpdatePageOCR(doc, pageNum, cached.OCRText)
			if cached.AIText != "" {
				a.pdfProcessor.UpdatePageAI(doc, pageNum, cached.AIText)
			}
			if cached.OriginalText != "" {
				a.pdfProcessor.UpdatePageText(doc, pageNum, cached.OriginalText)
			}

			// 即使从缓存加载，也要保存到历史记录
			if historyRecord != nil {
				var originalText string
				if pageNum > 0 && pageNum <= len(doc.Pages) {
					originalText = doc.Pages[pageNum-1].Text
				}

				page := &history.HistoryPage{
					HistoryID:      historyRecord.ID,
					PageNumber:     pageNum,
					OriginalText:   originalText,
					OCRText:        cached.OCRText,
					AIProcessedText: cached.AIText,
					ProcessingTime: time.Since(startTime).Seconds(),
				}

				log.Printf("保存缓存页面到历史记录: 页面%d, OCR长度=%d, AI长度=%d",
					pageNum, len(cached.OCRText), len(cached.AIText))

				if err := a.historyManager.AddPage(page); err != nil {
					log.Printf("保存历史记录失败: %v", err)
				} else {
					log.Printf("缓存页面历史记录保存成功: 页面%d", pageNum)
				}
			}

			return ProcessResult{
				PageNumber: pageNum,
				Status:     "从缓存加载",
				Error:      nil,
			}
		}
	}

	// 处理页面
	err := a.processSinglePage(pageNum, historyRecord)
	status := "处理完成"
	if err != nil {
		status = "处理失败"
	}

	return ProcessResult{
		PageNumber: pageNum,
		Status:     status,
		Error:      err,
	}
}

// GetSupportedFormats 获取支持的文档格式
func (a *App) GetSupportedFormats() []string {
	return a.documentProcessor.GetSupportedFormats()
}

// GetAppVersion 获取应用版本信息
func (a *App) GetAppVersion() map[string]string {
	return GetAppInfo()
}

// CheckSystemDependencies 检查系统依赖
func (a *App) CheckSystemDependencies() *system.SystemInfo {
	return system.CheckDependencies()
}

// GetInstallInstructions 获取依赖安装说明
func (a *App) GetInstallInstructions() map[string]string {
	return system.GetInstallInstructions()
}

// GetDocumentInfo 获取文档信息
func (a *App) GetDocumentInfo(filePath string) (*document.DocumentInfo, error) {
	return a.documentProcessor.GetDocumentInfo(filePath)
}

// GetSupportedModels 获取支持的AI模型
func (a *App) GetSupportedModels() []ocr.ModelInfo {
	if a.ocrClient != nil {
		return a.ocrClient.GetSupportedModels()
	}

	// 返回默认模型列表
	return []ocr.ModelInfo{
		{
			ID:             "gpt-4-vision-preview",
			Name:           "GPT-4 Vision Preview",
			Description:    "GPT-4的视觉预览版本，支持图片和文本处理",
			SupportsVision: true,
			MaxTokens:      4096,
			Recommended:    true,
		},
	}
}

// TestAIConnection 测试AI连接
func (a *App) TestAIConnection() error {
	if a.ocrClient == nil {
		return fmt.Errorf("未配置AI服务")
	}

	// 创建一个简单的测试
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 使用AI处理一个简单的文本测试
	_, err := a.ocrClient.ProcessWithAI(ctx, "测试连接", "请回复'连接成功'")
	if err != nil {
		return fmt.Errorf("AI连接测试失败: %w", err)
	}

	return nil
}

// GetProcessingStats 获取处理统计信息
func (a *App) GetProcessingStats() map[string]interface{} {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	stats := map[string]interface{}{
		"total_pages":     0,
		"processed_pages": 0,
		"cached_pages":    0,
		"has_document":    false,
	}

	if doc != nil {
		stats["has_document"] = true
		stats["total_pages"] = doc.PageCount

		processed := 0
		cached := 0

		for i, page := range doc.Pages {
			if page.Processed {
				processed++
			}

			// 检查缓存
			if a.checkPageCache(i+1) != nil {
				cached++
			}
		}

		stats["processed_pages"] = processed
		stats["cached_pages"] = cached
	}

	return stats
}
