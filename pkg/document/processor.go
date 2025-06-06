package document

import (
	"fmt"
	"path/filepath"
	"strings"

	"pdf-ocr-ai/pkg/pdf"
)

// DocumentType 文档类型
type DocumentType string

const (
	TypePDF   DocumentType = "pdf"
	TypeImage DocumentType = "image"
	TypeWord  DocumentType = "word"
	TypeText  DocumentType = "text"
)

// SupportedFormats 支持的文件格式
var SupportedFormats = map[string]DocumentType{
	".pdf":  TypePDF,
	".jpg":  TypeImage,
	".jpeg": TypeImage,
	".png":  TypeImage,
	".bmp":  TypeImage,
	".tiff": TypeImage,
	".tif":  TypeImage,
	".gif":  TypeImage,
	".webp": TypeImage,
	".doc":  TypeWord,
	".docx": TypeWord,
	".txt":  TypeText,
	".md":   TypeText,
	".rtf":  TypeText,
}

// DocumentInfo 文档信息
type DocumentInfo struct {
	FilePath     string       `json:"file_path"`
	Type         DocumentType `json:"type"`
	PageCount    int          `json:"page_count"`
	Title        string       `json:"title"`
	Author       string       `json:"author"`
	Subject      string       `json:"subject"`
	SupportedOCR bool         `json:"supported_ocr"`
}

// DocumentProcessor 文档处理器
type DocumentProcessor struct {
	pdfProcessor *pdf.PDFProcessor
}

// NewDocumentProcessor 创建文档处理器
func NewDocumentProcessor() (*DocumentProcessor, error) {
	pdfProcessor, err := pdf.NewPDFProcessor()
	if err != nil {
		return nil, fmt.Errorf("创建PDF处理器失败: %w", err)
	}

	return &DocumentProcessor{
		pdfProcessor: pdfProcessor,
	}, nil
}

// GetDocumentType 获取文档类型
func (dp *DocumentProcessor) GetDocumentType(filePath string) (DocumentType, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if docType, exists := SupportedFormats[ext]; exists {
		return docType, nil
	}
	return "", fmt.Errorf("不支持的文件格式: %s", ext)
}

// IsSupported 检查文件是否支持
func (dp *DocumentProcessor) IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	_, exists := SupportedFormats[ext]
	return exists
}

// GetDocumentInfo 获取文档信息
func (dp *DocumentProcessor) GetDocumentInfo(filePath string) (*DocumentInfo, error) {
	docType, err := dp.GetDocumentType(filePath)
	if err != nil {
		return nil, err
	}

	info := &DocumentInfo{
		FilePath:     filePath,
		Type:         docType,
		SupportedOCR: dp.supportsOCR(docType),
	}

	switch docType {
	case TypePDF:
		return dp.getPDFInfo(filePath, info)
	case TypeImage:
		return dp.getImageInfo(filePath, info)
	case TypeWord:
		return dp.getWordInfo(filePath, info)
	case TypeText:
		return dp.getTextInfo(filePath, info)
	default:
		return info, nil
	}
}

// supportsOCR 检查文档类型是否支持OCR
func (dp *DocumentProcessor) supportsOCR(docType DocumentType) bool {
	switch docType {
	case TypePDF, TypeImage:
		return true
	case TypeWord, TypeText:
		return false // 这些格式已经包含文本，不需要OCR
	default:
		return false
	}
}

// getPDFInfo 获取PDF文档信息
func (dp *DocumentProcessor) getPDFInfo(filePath string, info *DocumentInfo) (*DocumentInfo, error) {
	doc, err := dp.pdfProcessor.LoadPDF(filePath)
	if err != nil {
		return nil, fmt.Errorf("加载PDF失败: %w", err)
	}

	info.PageCount = doc.PageCount
	info.Title = doc.Title
	info.Author = doc.Author
	info.Subject = doc.Subject

	return info, nil
}

// getImageInfo 获取图片文档信息
func (dp *DocumentProcessor) getImageInfo(filePath string, info *DocumentInfo) (*DocumentInfo, error) {
	// 图片文件只有一页
	info.PageCount = 1
	info.Title = filepath.Base(filePath)

	return info, nil
}

// getWordInfo 获取Word文档信息
func (dp *DocumentProcessor) getWordInfo(filePath string, info *DocumentInfo) (*DocumentInfo, error) {
	// Word文档处理（需要额外的库支持）
	info.PageCount = 1 // 简化处理，假设为1页
	info.Title = filepath.Base(filePath)
	info.SupportedOCR = false // Word文档已包含文本

	return info, nil
}

// getTextInfo 获取文本文档信息
func (dp *DocumentProcessor) getTextInfo(filePath string, info *DocumentInfo) (*DocumentInfo, error) {
	// 文本文件处理
	info.PageCount = 1
	info.Title = filepath.Base(filePath)
	info.SupportedOCR = false // 文本文件已包含文本

	return info, nil
}

// LoadDocument 加载文档
func (dp *DocumentProcessor) LoadDocument(filePath string) (*pdf.PDFDocument, error) {
	docType, err := dp.GetDocumentType(filePath)
	if err != nil {
		return nil, err
	}

	switch docType {
	case TypePDF:
		return dp.pdfProcessor.LoadPDF(filePath)
	case TypeImage:
		return dp.loadImageAsDocument(filePath)
	case TypeWord:
		return dp.loadWordAsDocument(filePath)
	case TypeText:
		return dp.loadTextAsDocument(filePath)
	default:
		return nil, fmt.Errorf("不支持的文档类型: %s", docType)
	}
}

// loadImageAsDocument 将图片加载为文档
func (dp *DocumentProcessor) loadImageAsDocument(filePath string) (*pdf.PDFDocument, error) {
	// 创建一个虚拟的PDF文档结构来表示图片
	doc := &pdf.PDFDocument{
		FilePath:  filePath,
		PageCount: 1,
		Title:     filepath.Base(filePath),
		Pages: []*pdf.PDFPage{
			{
				Number:    1,
				Text:      "", // 图片没有原生文本
				HasText:   false,
				Width:     0, // 需要从图片获取实际尺寸
				Height:    0,
				ImagePath: filePath, // 直接使用原图片路径
			},
		},
	}

	return doc, nil
}

// loadWordAsDocument 将Word文档加载为文档
func (dp *DocumentProcessor) loadWordAsDocument(filePath string) (*pdf.PDFDocument, error) {
	// Word文档处理（简化实现）
	doc := &pdf.PDFDocument{
		FilePath:  filePath,
		PageCount: 1,
		Title:     filepath.Base(filePath),
		Pages: []*pdf.PDFPage{
			{
				Number:  1,
				Text:    "Word文档内容需要专门的解析器", // 实际实现需要Word解析库
				HasText: true,
				Width:   595,
				Height:  842,
			},
		},
	}

	return doc, nil
}

// loadTextAsDocument 将文本文件加载为文档
func (dp *DocumentProcessor) loadTextAsDocument(filePath string) (*pdf.PDFDocument, error) {
	// 文本文件处理（简化实现）
	doc := &pdf.PDFDocument{
		FilePath:  filePath,
		PageCount: 1,
		Title:     filepath.Base(filePath),
		Pages: []*pdf.PDFPage{
			{
				Number:  1,
				Text:    "文本文件内容需要读取文件", // 实际实现需要读取文件内容
				HasText: true,
				Width:   595,
				Height:  842,
			},
		},
	}

	return doc, nil
}

// GetSupportedFormats 获取支持的格式列表
func (dp *DocumentProcessor) GetSupportedFormats() []string {
	formats := make([]string, 0, len(SupportedFormats))
	for ext := range SupportedFormats {
		formats = append(formats, ext)
	}
	return formats
}

// GetFormatDescription 获取格式描述
func (dp *DocumentProcessor) GetFormatDescription(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".pdf":
		return "PDF文档"
	case ".jpg", ".jpeg":
		return "JPEG图片"
	case ".png":
		return "PNG图片"
	case ".bmp":
		return "BMP图片"
	case ".tiff", ".tif":
		return "TIFF图片"
	case ".gif":
		return "GIF图片"
	case ".webp":
		return "WebP图片"
	case ".doc":
		return "Word文档 (旧版)"
	case ".docx":
		return "Word文档"
	case ".txt":
		return "纯文本文件"
	case ".md":
		return "Markdown文件"
	case ".rtf":
		return "富文本格式"
	default:
		return "未知格式"
	}
}

// Cleanup 清理资源
func (dp *DocumentProcessor) Cleanup() error {
	if dp.pdfProcessor != nil {
		return dp.pdfProcessor.Cleanup()
	}
	return nil
}
