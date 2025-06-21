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

	"pdf-ocr-ai/pkg/cache"
	"pdf-ocr-ai/pkg/config"
	"pdf-ocr-ai/pkg/document"
	"pdf-ocr-ai/pkg/history"
	"pdf-ocr-ai/pkg/ocr"
	"pdf-ocr-ai/pkg/pdf"
	"pdf-ocr-ai/pkg/system"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ProgressUpdate 进度更新
type ProgressUpdate struct {
	Total       int    `json:"total"`
	Processed   int    `json:"processed"`
	CurrentPage int    `json:"current_page"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// ProcessingState 处理状态
type ProcessingState int

const (
	ProcessingStateIdle ProcessingState = iota
	ProcessingStateRunning
	ProcessingStatePaused
	ProcessingStateCancelling
)

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
	// 批量处理控制
	processingCancel context.CancelFunc
	processingMu     sync.Mutex
	processingState  ProcessingState
	pauseSignal      chan bool
	resumeSignal     chan bool
	currentBatch     []int // 当前批次的页面
	processedInBatch int   // 当前批次已处理的页面数
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

// ProcessSinglePage 处理单个页面（非批量）
func (a *App) ProcessSinglePage(pageNumber int) {
	go a.processSinglePageWithHistory(pageNumber, false)
}

// ProcessSinglePageForce 强制处理单个页面（非批量）
func (a *App) ProcessSinglePageForce(pageNumber int) {
	go a.processSinglePageWithHistory(pageNumber, true)
}

// processSinglePageWithHistory 处理单个页面并创建历史记录
func (a *App) processSinglePageWithHistory(pageNumber int, forceReprocess bool) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		runtime.EventsEmit(a.ctx, "processing-error", "未加载PDF文档")
		return
	}

	if a.ocrClient == nil {
		runtime.EventsEmit(a.ctx, "processing-error", "未配置AI服务")
		return
	}

	// 获取实际使用的OCR模型名称
	aiConfig := a.configManager.GetAIConfig()
	actualOCRModel := aiConfig.OCRModel
	if actualOCRModel == "" {
		actualOCRModel = aiConfig.Model
	}

	// 创建历史记录，使用实际的OCR模型名称（OCR任务不添加前缀）
	historyRecord, err := a.historyManager.CreateRecord(doc.FilePath, 1, actualOCRModel)
	if err != nil {
		log.Printf("创建单页OCR历史记录失败: %v", err)
	}

	// 创建上下文
	ctx := context.Background()

	// 处理页面
	err = a.processSinglePage(ctx, pageNumber, historyRecord)
	if err != nil {
		log.Printf("单页OCR处理失败: %v", err)
		if historyRecord != nil {
			a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusFailed, err.Error())
		}
		runtime.EventsEmit(a.ctx, "processing-error", fmt.Sprintf("处理第%d页失败: %v", pageNumber, err))
		return
	}

	// 更新历史记录状态
	if historyRecord != nil {
		a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCompleted, "")
	}

	// 发送单页完成事件
	runtime.EventsEmit(a.ctx, "page-processed", map[string]interface{}{
		"pageNumber": pageNumber,
		"status":     "处理完成",
	})

	log.Printf("单页OCR处理完成: 页面%d", pageNumber)
}

// ProcessPagesForce 强制重新处理指定页面（跳过缓存）
func (a *App) ProcessPagesForce(pageNumbers []int) {
	go a.processPagesBatch(pageNumbers, true)
}

// PauseProcessing 暂停当前的批量处理
func (a *App) PauseProcessing() {
	a.processingMu.Lock()
	defer a.processingMu.Unlock()

	if a.processingState == ProcessingStateRunning {
		log.Printf("用户请求暂停批量处理")
		a.processingState = ProcessingStatePaused

		// 发送暂停信号
		select {
		case a.pauseSignal <- true:
		default:
		}

		// 发送暂停通知
		runtime.EventsEmit(a.ctx, "processing-paused", map[string]interface{}{
			"message": "批量处理已暂停",
		})
	}
}

// ResumeProcessing 继续当前的批量处理
func (a *App) ResumeProcessing() {
	a.processingMu.Lock()
	defer a.processingMu.Unlock()

	if a.processingState == ProcessingStatePaused {
		log.Printf("用户请求继续批量处理")
		a.processingState = ProcessingStateRunning

		// 发送继续信号
		select {
		case a.resumeSignal <- true:
		default:
		}

		// 发送继续通知
		runtime.EventsEmit(a.ctx, "processing-resumed", map[string]interface{}{
			"message": "批量处理已继续",
		})
	}
}

// CancelProcessing 取消当前的批量处理
func (a *App) CancelProcessing() {
	a.processingMu.Lock()
	defer a.processingMu.Unlock()

	if a.processingState == ProcessingStateRunning || a.processingState == ProcessingStatePaused {
		log.Printf("用户请求取消批量处理")
		a.processingState = ProcessingStateCancelling

		if a.processingCancel != nil {
			a.processingCancel()
		}

		// 发送取消通知，但不立即清理状态，让批量处理函数自己清理
		runtime.EventsEmit(a.ctx, "processing-cancelled", map[string]interface{}{
			"message": "批量处理已取消",
		})
	}
}

// GetProcessingState 获取当前处理状态
func (a *App) GetProcessingState() map[string]interface{} {
	a.processingMu.Lock()
	defer a.processingMu.Unlock()

	return map[string]interface{}{
		"state":           int(a.processingState),
		"current_batch":   a.currentBatch,
		"processed_count": a.processedInBatch,
		"total_count":     len(a.currentBatch),
	}
}

// CheckProcessedPages 检查哪些页面已经处理过
func (a *App) CheckProcessedPages(pageNumbers []int) map[string]interface{} {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	result := map[string]interface{}{
		"total_pages":       len(pageNumbers),
		"processed_pages":   []int{},
		"unprocessed_pages": []int{},
		"processed_count":   0,
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

	// 初始化处理状态
	a.processingMu.Lock()
	processingCtx, cancel := context.WithCancel(a.ctx)
	a.processingCancel = cancel
	a.processingState = ProcessingStateRunning
	a.currentBatch = pageNumbers
	a.processedInBatch = 0

	// 初始化信号通道
	if a.pauseSignal == nil {
		a.pauseSignal = make(chan bool, 1)
	}
	if a.resumeSignal == nil {
		a.resumeSignal = make(chan bool, 1)
	}
	a.processingMu.Unlock()

	// 确保在函数结束时清理
	defer func() {
		a.processingMu.Lock()
		if a.processingCancel != nil {
			a.processingCancel = nil
		}
		a.processingState = ProcessingStateIdle
		a.currentBatch = nil
		a.processedInBatch = 0
		a.processingMu.Unlock()
		cancel()
	}()

	// 获取实际使用的OCR模型名称
	aiConfig := a.configManager.GetAIConfig()
	actualOCRModel := aiConfig.OCRModel
	if actualOCRModel == "" {
		actualOCRModel = aiConfig.Model
	}

	// 创建历史记录，使用实际的OCR模型名称（OCR任务不添加前缀）
	historyRecord, err := a.historyManager.CreateRecord(doc.FilePath, len(pageNumbers), actualOCRModel)
	if err != nil {
		log.Printf("创建历史记录失败: %v", err)
	}

	// 发送初始进度
	runtime.EventsEmit(a.ctx, "processing-progress", ProgressUpdate{
		Total:     len(pageNumbers),
		Processed: 0,
		Status:    "开始处理",
	})

	// 使用并发处理（传入可取消的上下文）
	processed := a.processPagesConcurrently(processingCtx, pageNumbers, historyRecord, doc, forceReprocess)

	// 检查上下文是否被取消
	select {
	case <-processingCtx.Done():
		log.Printf("批量处理被取消")
		if historyRecord != nil {
			a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCancelled, "处理被用户取消")
		}
		return
	default:
		// 正常完成
	}

	// 更新历史记录状态
	if historyRecord != nil {
		a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCompleted, "")
	}

	// 发送完成通知
	runtime.EventsEmit(a.ctx, "processing-complete", map[string]interface{}{
		"total_processed": processed,
		"document":        doc,
		"processedPages":  pageNumbers, // 添加处理过的页面信息
	})
}

// processSinglePage 处理单个页面
func (a *App) processSinglePage(ctx context.Context, pageNum int, historyRecord *history.HistoryRecord) error {
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

	// 检查是否被取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 使用AI识别文字（带重试机制）
	log.Printf("开始OCR识别页面 %d", pageNum)
	result, err := a.ocrClient.RecognizeImage(ctx, imagePath)
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

// ProcessWithAI 使用AI处理文本（不支持上下文模式，保持向后兼容）
func (a *App) ProcessWithAI(pageNumbers []int, prompt string) {
	go a.processWithAI(pageNumbers, prompt, false)
}

// ProcessWithAIContext 使用AI处理文本（支持上下文模式）
func (a *App) ProcessWithAIContext(pageNumbers []int, prompt string, contextMode bool) {
	go a.processWithAI(pageNumbers, prompt, contextMode)
}

// processWithAI AI处理文本
func (a *App) processWithAI(pageNumbers []int, prompt string, contextMode bool) {
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

	// 获取实际使用的AI文本处理模型名称
	aiConfig := a.configManager.GetAIConfig()
	actualAIModel := aiConfig.TextModel
	if actualAIModel == "" {
		// 如果没有设置文本处理模型，回退到通用模型
		actualAIModel = aiConfig.Model
		if actualAIModel == "" {
			actualAIModel = "未知模型"
		}
	}

	// 创建历史记录，使用实际的AI模型名称，添加AI前缀标识任务类型
	historyRecord, err := a.historyManager.CreateRecord(doc.FilePath, len(pageNumbers), "AI-"+actualAIModel)
	if err != nil {
		log.Printf("创建AI处理历史记录失败: %v", err)
	}

	// 根据上下文模式选择处理方式
	if contextMode && len(pageNumbers) == 1 {
		// 上下文模式且单页处理：使用新的单页AI处理逻辑
		pageNum := pageNumbers[0]
		result := a.processPageAI(context.Background(), pageNum, prompt, doc, false, contextMode, historyRecord)

		if result.Error != nil {
			// 更新历史记录状态为失败
			if historyRecord != nil {
				a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusFailed, fmt.Sprintf("AI处理失败: %v", result.Error))
			}
			runtime.EventsEmit(a.ctx, "ai-processing-error", fmt.Sprintf("AI处理失败: %v", result.Error))
			return
		}

		// 更新历史记录状态为完成
		if historyRecord != nil {
			a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCompleted, "")
		}

		// 发送结果
		runtime.EventsEmit(a.ctx, "ai-processing-complete", map[string]interface{}{
			"pages":  pageNumbers,
			"prompt": prompt,
			"result": result.Result,
		})
		return
	}

	// 传统模式：收集所有页面文本合并处理
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

		// 保存到历史记录
		if historyRecord != nil {
			historyPage := &history.HistoryPage{
				HistoryID:       historyRecord.ID,
				PageNumber:      pageNum,
				OriginalText:    page.Text,
				OCRText:         page.OCRText,
				AIProcessedText: result,
				ProcessingTime:  0, // 非批量处理暂时设为0
			}
			if err := a.historyManager.AddPage(historyPage); err != nil {
				log.Printf("保存AI处理历史记录失败: %v", err)
			} else {
				log.Printf("AI处理历史记录保存成功: 页面%d", pageNum)
			}
		}
	}

	// 更新历史记录状态
	if historyRecord != nil {
		a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCompleted, "")
	}

	// 发送结果
	runtime.EventsEmit(a.ctx, "ai-processing-complete", map[string]interface{}{
		"pages":  pageNumbers,
		"prompt": prompt,
		"result": result,
	})
}

// CheckAIProcessedPages 检查页面AI处理状态
func (a *App) CheckAIProcessedPages(pageNumbers []int) map[string]interface{} {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	result := map[string]interface{}{
		"total_pages":       len(pageNumbers),
		"processed_pages":   []int{},
		"unprocessed_pages": []int{},
		"processed_count":   0,
	}

	if doc == nil {
		return result
	}

	processedPages := []int{}
	unprocessedPages := []int{}

	for _, pageNum := range pageNumbers {
		if pageNum < 1 || pageNum > len(doc.Pages) {
			continue
		}

		page := doc.Pages[pageNum-1]
		if page.AIText != "" {
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

// ProcessWithAIBatch 批量AI处理（每页单独处理）
func (a *App) ProcessWithAIBatch(pageNumbers []int, prompt string) {
	go a.processWithAIBatch(pageNumbers, prompt, false, false)
}

// ProcessWithAIBatchForce 强制批量AI处理（忽略缓存）
func (a *App) ProcessWithAIBatchForce(pageNumbers []int, prompt string) {
	go a.processWithAIBatch(pageNumbers, prompt, true, false)
}

// ProcessWithAIBatchContext 批量AI处理（支持上下文模式）
func (a *App) ProcessWithAIBatchContext(pageNumbers []int, prompt string, contextMode bool) {
	go a.processWithAIBatch(pageNumbers, prompt, false, contextMode)
}

// ProcessWithAIBatchForceContext 强制批量AI处理（支持上下文模式）
func (a *App) ProcessWithAIBatchForceContext(pageNumbers []int, prompt string, contextMode bool) {
	go a.processWithAIBatch(pageNumbers, prompt, true, contextMode)
}

// processWithAIBatch 批量AI处理实现
func (a *App) processWithAIBatch(pageNumbers []int, prompt string, forceReprocess bool, contextMode bool) {
	a.mu.RLock()
	doc := a.currentDoc
	a.mu.RUnlock()

	if doc == nil {
		runtime.EventsEmit(a.ctx, "processing-error", "未加载PDF文档")
		return
	}

	if a.ocrClient == nil {
		runtime.EventsEmit(a.ctx, "processing-error", "未配置AI服务")
		return
	}

	// 过滤有效页面
	validPages := []int{}
	for _, pageNum := range pageNumbers {
		if pageNum >= 1 && pageNum <= len(doc.Pages) {
			page := doc.Pages[pageNum-1]
			if page.OCRText != "" || page.Text != "" {
				validPages = append(validPages, pageNum)
			}
		}
	}

	if len(validPages) == 0 {
		runtime.EventsEmit(a.ctx, "processing-error", "没有可处理的页面")
		return
	}

	// 获取实际使用的AI文本处理模型名称
	aiConfig := a.configManager.GetAIConfig()
	actualAIModel := aiConfig.TextModel
	if actualAIModel == "" {
		// 如果没有设置文本处理模型，回退到通用模型
		actualAIModel = aiConfig.Model
		if actualAIModel == "" {
			actualAIModel = "未知模型"
		}
	}

	// 创建历史记录，使用实际的AI模型名称，添加AI前缀标识任务类型
	historyRecord, err := a.historyManager.CreateRecord(doc.FilePath, len(validPages), "AI-"+actualAIModel)
	if err != nil {
		log.Printf("创建AI处理历史记录失败: %v", err)
	}

	// 创建上下文用于取消
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置处理状态
	a.processingMu.Lock()
	a.processingState = 1 // processing
	a.processedInBatch = 0
	a.processingMu.Unlock()

	// 并发处理AI任务
	const maxConcurrency = 2 // AI处理并发数较低，避免API限制
	pagesChan := make(chan int, len(validPages))
	resultsChan := make(chan AIProcessResult, len(validPages))

	// 发送页面到通道
	for _, pageNum := range validPages {
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
				// 检查取消状态
				select {
				case <-ctx.Done():
					log.Printf("AI处理协程检测到取消信号，停止处理")
					return
				default:
				}

				result := a.processPageAI(ctx, pageNum, prompt, doc, forceReprocess, contextMode, historyRecord)

				select {
				case <-ctx.Done():
					return
				case resultsChan <- result:
				}
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
	total := len(validPages)
	successCount := 0

	for result := range resultsChan {
		processed++

		if result.Error != nil {
			log.Printf("AI处理第%d页失败: %v", result.PageNumber, result.Error)
			// 检查是否是取消导致的错误
			if result.Error == context.Canceled || strings.Contains(result.Error.Error(), "context canceled") {
				log.Printf("页面 %d AI处理被取消", result.PageNumber)
			} else {
				runtime.EventsEmit(a.ctx, "processing-error", fmt.Sprintf("AI处理第%d页失败: %v", result.PageNumber, result.Error))
			}
		} else {
			successCount++
			// AI页面处理成功，立即发送单页完成事件以触发实时刷新
			runtime.EventsEmit(a.ctx, "ai-page-processed", map[string]interface{}{
				"pageNumber": result.PageNumber,
				"status":     result.Status,
				"result":     result.Result,
			})
		}

		runtime.EventsEmit(a.ctx, "processing-progress", ProgressUpdate{
			Total:       total,
			Processed:   processed,
			CurrentPage: result.PageNumber,
			Status:      result.Status,
		})
	}

	// 重置处理状态
	a.processingMu.Lock()
	a.processingState = 0 // idle
	a.processingMu.Unlock()

	// 更新历史记录状态
	if historyRecord != nil {
		if successCount > 0 {
			a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusCompleted, "")
		} else {
			a.historyManager.UpdateRecordStatus(historyRecord.ID, history.StatusFailed, "所有页面处理失败")
		}
	}

	// 发送完成事件
	if successCount > 0 {
		runtime.EventsEmit(a.ctx, "ai-processing-complete", map[string]interface{}{
			"pages":        validPages,
			"prompt":       prompt,
			"successCount": successCount,
			"totalCount":   total,
		})
	}
}

// processPageAI 处理单个页面的AI任务
func (a *App) processPageAI(ctx context.Context, pageNum int, prompt string, doc *pdf.PDFDocument, forceReprocess bool, contextMode bool, historyRecord *history.HistoryRecord) AIProcessResult {
	startTime := time.Now()
	result := AIProcessResult{
		PageNumber: pageNum,
		Status:     fmt.Sprintf("正在AI处理第%d页", pageNum),
	}

	// 检查页面范围
	if pageNum < 1 || pageNum > len(doc.Pages) {
		result.Error = fmt.Errorf("页码超出范围")
		return result
	}

	page := doc.Pages[pageNum-1]

	// 获取上下文内容（包含当前页面和前后页面的内容）
	currentPageText, _, _, contextPrompt := a.collectContextContent(doc, pageNum, contextMode)

	if currentPageText == "" {
		result.Error = fmt.Errorf("页面没有可处理的文本")
		return result
	}

	// 根据上下文模式设置处理文本
	var processText string
	var finalPrompt string
	if contextMode {
		// 上下文模式：只发送当前页内容给AI，但在提示词中包含上下文信息
		processText = currentPageText
		finalPrompt = contextPrompt + "【重要提示】请严格按照以下指令处理上述第" + fmt.Sprintf("%d", pageNum) + "页的内容，不要包含其他页面的内容：\n\n" + prompt
	} else {
		// 普通模式：只使用当前页面内容
		processText = currentPageText
		finalPrompt = prompt
	}

	// 检查缓存（只有在强制重新处理时才跳过缓存）
	// 注意：单页AI处理通常是用户主动触发的，可能使用不同的提示词或上下文模式
	// 因此我们不使用缓存，总是进行新的AI处理
	if !forceReprocess && page.AIText != "" {
		log.Printf("第%d页已有AI处理结果，但单页AI处理总是使用新的提示词，跳过缓存")
	}

	log.Printf("开始AI处理第%d页", pageNum)

	// 使用AI处理（使用上下文内容）
	aiResult, err := a.ocrClient.ProcessWithAI(ctx, processText, finalPrompt)
	if err != nil {
		result.Error = fmt.Errorf("AI处理失败: %w", err)
		return result
	}

	// 更新页面AI处理结果
	a.pdfProcessor.UpdatePageAI(doc, pageNum, aiResult)

	// 保存到缓存
	if err := a.savePageToCache(pageNum, page.OCRText, aiResult); err != nil {
		log.Printf("保存AI处理结果到缓存失败: %v", err)
	}

	// 保存到历史记录
	if historyRecord != nil {
		historyPage := &history.HistoryPage{
			HistoryID:       historyRecord.ID,
			PageNumber:      pageNum,
			OriginalText:    page.Text,
			OCRText:         page.OCRText,
			AIProcessedText: aiResult,
			ProcessingTime:  time.Since(startTime).Seconds(),
		}
		if err := a.historyManager.AddPage(historyPage); err != nil {
			log.Printf("保存AI处理历史记录失败: %v", err)
		} else {
			log.Printf("AI处理历史记录保存成功: 页面%d", pageNum)
		}
	}

	result.Result = aiResult
	result.Status = fmt.Sprintf("第%d页AI处理完成", pageNum)

	log.Printf("第%d页AI处理完成", pageNum)
	return result
}

// collectContextContent 收集上下文内容（当前页面的前后页面内容）
func (a *App) collectContextContent(doc *pdf.PDFDocument, currentPageNum int, contextMode bool) (string, string, string, string) {
	// 检查是否启用上下文模式
	if !contextMode {
		// 未启用上下文模式，只返回当前页面内容
		if currentPageNum >= 1 && currentPageNum <= len(doc.Pages) {
			page := doc.Pages[currentPageNum-1]
			text := page.OCRText
			if text == "" {
				text = page.Text
			}
			return text, "", "", ""
		}
		return "", "", "", ""
	}

	// 启用上下文模式，分别收集上一页、当前页、下一页内容
	log.Printf("启用上下文模式，为第%d页收集上下文内容", currentPageNum)

	var prevPageText, currentPageText, nextPageText string

	// 获取上一页内容
	if currentPageNum > 1 && currentPageNum-1 <= len(doc.Pages) {
		page := doc.Pages[currentPageNum-2]
		text := page.OCRText
		if text == "" {
			text = page.Text
		}
		if strings.TrimSpace(text) != "" {
			prevPageText = text
		}
	}

	// 获取当前页内容
	if currentPageNum >= 1 && currentPageNum <= len(doc.Pages) {
		page := doc.Pages[currentPageNum-1]
		text := page.OCRText
		if text == "" {
			text = page.Text
		}
		currentPageText = text
	}

	// 获取下一页内容
	if currentPageNum < len(doc.Pages) {
		page := doc.Pages[currentPageNum]
		text := page.OCRText
		if text == "" {
			text = page.Text
		}
		if strings.TrimSpace(text) != "" {
			nextPageText = text
		}
	}

	// 构建上下文提示
	var contextPrompt strings.Builder
	contextPrompt.WriteString("【上下文信息】\n")

	if prevPageText != "" {
		contextPrompt.WriteString(fmt.Sprintf("上一页（第%d页）内容：\n%s\n\n", currentPageNum-1, prevPageText))
	} else {
		if currentPageNum == 1 {
			contextPrompt.WriteString("上一页：无（当前是第一页）\n\n")
		} else {
			contextPrompt.WriteString("上一页：无内容或为空页\n\n")
		}
	}

	contextPrompt.WriteString(fmt.Sprintf("当前页（第%d页）内容：\n%s\n\n", currentPageNum, currentPageText))

	if nextPageText != "" {
		contextPrompt.WriteString(fmt.Sprintf("下一页（第%d页）内容：\n%s\n\n", currentPageNum+1, nextPageText))
	} else {
		if currentPageNum == len(doc.Pages) {
			contextPrompt.WriteString("下一页：无（当前是最后一页）\n\n")
		} else {
			contextPrompt.WriteString("下一页：无内容或为空页\n\n")
		}
	}

	contextPrompt.WriteString("【处理要求】\n")
	contextPrompt.WriteString(fmt.Sprintf("请根据上述上下文信息处理第%d页的内容。\n", currentPageNum))
	contextPrompt.WriteString("⚠️ 重要限制：\n")
	contextPrompt.WriteString("1. 只处理和输出第" + fmt.Sprintf("%d", currentPageNum) + "页的内容\n")
	contextPrompt.WriteString("2. 上一页和下一页的内容仅作为理解上下文的参考，不要在输出中包含它们的内容\n")
	contextPrompt.WriteString("3. 如果当前页内容与前后页有连续性，可以适当提及相关背景，但主体内容必须是当前页\n")
	contextPrompt.WriteString("4. 严格按照页面边界进行处理，避免跨页面混合内容\n\n")

	log.Printf("为第%d页收集的上下文内容长度: %d", currentPageNum, len(contextPrompt.String()))

	// 调试日志：输出收集到的内容
	log.Printf("=== 调试信息：第%d页上下文收集 ===", currentPageNum)
	log.Printf("上一页内容长度: %d", len(prevPageText))
	if prevPageText != "" {
		log.Printf("上一页内容前100字符: %s", truncateString(prevPageText, 100))
	}
	log.Printf("当前页内容长度: %d", len(currentPageText))
	if currentPageText != "" {
		log.Printf("当前页内容前100字符: %s", truncateString(currentPageText, 100))
	}
	log.Printf("下一页内容长度: %d", len(nextPageText))
	if nextPageText != "" {
		log.Printf("下一页内容前100字符: %s", truncateString(nextPageText, 100))
	}
	log.Printf("=== 调试信息结束 ===")

	return currentPageText, prevPageText, nextPageText, contextPrompt.String()
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
func (a *App) processPagesConcurrently(ctx context.Context, pageNumbers []int, historyRecord *history.HistoryRecord, doc *pdf.PDFDocument, forceReprocess bool) int {
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
				// 检查暂停/取消状态
				for {
					a.processingMu.Lock()
					state := a.processingState
					a.processingMu.Unlock()

					if state == ProcessingStateCancelling {
						log.Printf("工作协程检测到取消信号，停止处理")
						return
					}

					if state == ProcessingStatePaused {
						log.Printf("工作协程检测到暂停信号，等待继续")
						// 等待继续信号或取消信号
						select {
						case <-ctx.Done():
							log.Printf("工作协程检测到上下文取消")
							return
						case <-a.resumeSignal:
							log.Printf("工作协程收到继续信号")
							break
						case <-a.pauseSignal:
							// 可能收到多个暂停信号，忽略
							continue
						}
					} else {
						break
					}
				}

				// 检查是否被取消
				select {
				case <-ctx.Done():
					log.Printf("工作协程检测到上下文取消，停止处理")
					return
				default:
				}

				result := a.processPageWithResult(ctx, pageNum, historyRecord, doc, forceReprocess)

				// 更新已处理计数
				a.processingMu.Lock()
				a.processedInBatch++
				a.processingMu.Unlock()

				// 再次检查是否被取消
				select {
				case <-ctx.Done():
					return
				case resultsChan <- result:
				}
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
			// 检查是否是取消导致的错误
			if result.Error == context.Canceled || strings.Contains(result.Error.Error(), "context canceled") {
				log.Printf("页面 %d 处理被取消", result.PageNumber)
				// 取消导致的错误不发送 processing-error 事件
			} else {
				// 只有真正的错误才发送 processing-error 事件
				runtime.EventsEmit(a.ctx, "processing-error", fmt.Sprintf("处理第%d页失败: %v", result.PageNumber, result.Error))
			}
		} else {
			// 页面处理成功，立即发送单页完成事件以触发实时刷新
			runtime.EventsEmit(a.ctx, "page-processed", map[string]interface{}{
				"pageNumber": result.PageNumber,
				"status":     result.Status,
			})
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

// AIProcessResult AI处理结果
type AIProcessResult struct {
	PageNumber int
	Status     string
	Result     string
	Error      error
}

// processPageWithResult 处理页面并返回结果
func (a *App) processPageWithResult(ctx context.Context, pageNum int, historyRecord *history.HistoryRecord, doc *pdf.PDFDocument, forceReprocess bool) ProcessResult {
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
					HistoryID:       historyRecord.ID,
					PageNumber:      pageNum,
					OriginalText:    originalText,
					OCRText:         cached.OCRText,
					AIProcessedText: cached.AIText,
					ProcessingTime:  time.Since(startTime).Seconds(),
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

	// 检查是否被取消
	select {
	case <-ctx.Done():
		return ProcessResult{
			PageNumber: pageNum,
			Status:     "处理被取消",
			Error:      ctx.Err(),
		}
	default:
	}

	// 处理页面
	err := a.processSinglePage(ctx, pageNum, historyRecord)
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
