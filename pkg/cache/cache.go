package cache

import (
	"crypto/md5"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	ID          int       `db:"id" json:"id"`
	DocumentID  string    `db:"document_id" json:"document_id"`
	PageNumber  int       `db:"page_number" json:"page_number"`
	OriginalText string   `db:"original_text" json:"original_text"`
	OCRText     string    `db:"ocr_text" json:"ocr_text"`
	AIText      string    `db:"ai_text" json:"ai_text"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// DocumentCache 文档缓存
type DocumentCache struct {
	ID         string    `db:"id" json:"id"`
	FilePath   string    `db:"file_path" json:"file_path"`
	FileHash   string    `db:"file_hash" json:"file_hash"`
	PageCount  int       `db:"page_count" json:"page_count"`
	Title      string    `db:"title" json:"title"`
	Author     string    `db:"author" json:"author"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// CacheManager 缓存管理器
type CacheManager struct {
	db *sqlx.DB
}

// NewCacheManager 创建缓存管理器
func NewCacheManager() (*CacheManager, error) {
	// 获取用户目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	// 创建数据目录
	dataDir := filepath.Join(homeDir, ".pdfSeer")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 连接数据库
	dbPath := filepath.Join(dataDir, "cache.db")
	db, err := sqlx.Connect("sqlite3", dbPath+"?cache=shared&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	cm := &CacheManager{db: db}

	// 初始化数据库表
	if err := cm.initTables(); err != nil {
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	return cm, nil
}

// initTables 初始化数据库表
func (cm *CacheManager) initTables() error {
	// 文档表
	documentsSQL := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		file_path TEXT UNIQUE NOT NULL,
		file_hash TEXT NOT NULL,
		page_count INTEGER NOT NULL,
		title TEXT,
		author TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// 页面缓存表
	pagesSQL := `
	CREATE TABLE IF NOT EXISTS pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id TEXT NOT NULL,
		page_number INTEGER NOT NULL,
		original_text TEXT,
		ocr_text TEXT,
		ai_text TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (document_id) REFERENCES documents(id),
		UNIQUE(document_id, page_number)
	);`

	// 创建索引
	indexSQL := `
	CREATE INDEX IF NOT EXISTS idx_pages_document_page ON pages(document_id, page_number);
	CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(file_hash);
	`

	// 执行SQL
	for _, sql := range []string{documentsSQL, pagesSQL, indexSQL} {
		if _, err := cm.db.Exec(sql); err != nil {
			return fmt.Errorf("执行SQL失败: %w", err)
		}
	}

	return nil
}

// GenerateDocumentID 生成文档ID
func (cm *CacheManager) GenerateDocumentID(filePath string) (string, error) {
	// 获取文件信息
	stat, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算文件hash
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	// 读取文件前1KB用于hash计算（避免大文件性能问题）
	buffer := make([]byte, 1024)
	n, _ := file.Read(buffer)
	hash.Write(buffer[:n])

	// 组合文件大小、修改时间和内容hash
	hashStr := fmt.Sprintf("%d-%d-%x", 
		stat.Size(), 
		stat.ModTime().UnixNano(), 
		hash.Sum(nil))

	return hashStr, nil
}

// SaveDocument 保存文档信息
func (cm *CacheManager) SaveDocument(doc *DocumentCache) error {
	query := `
	INSERT OR REPLACE INTO documents 
	(id, file_path, file_hash, page_count, title, author, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := cm.db.Exec(query, doc.ID, doc.FilePath, doc.FileHash, 
		doc.PageCount, doc.Title, doc.Author)
	
	return err
}

// GetDocument 获取文档信息
func (cm *CacheManager) GetDocument(documentID string) (*DocumentCache, error) {
	var doc DocumentCache
	query := `SELECT * FROM documents WHERE id = ?`
	
	err := cm.db.Get(&doc, query, documentID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	return &doc, err
}

// SavePage 保存页面缓存
func (cm *CacheManager) SavePage(entry *CacheEntry) error {
	query := `
	INSERT OR REPLACE INTO pages 
	(document_id, page_number, original_text, ocr_text, ai_text, updated_at)
	VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := cm.db.Exec(query, entry.DocumentID, entry.PageNumber,
		entry.OriginalText, entry.OCRText, entry.AIText)
	
	return err
}

// GetPage 获取页面缓存
func (cm *CacheManager) GetPage(documentID string, pageNumber int) (*CacheEntry, error) {
	var entry CacheEntry
	query := `SELECT * FROM pages WHERE document_id = ? AND page_number = ?`
	
	err := cm.db.Get(&entry, query, documentID, pageNumber)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	return &entry, err
}

// GetDocumentPages 获取文档所有页面
func (cm *CacheManager) GetDocumentPages(documentID string) ([]*CacheEntry, error) {
	var entries []*CacheEntry
	query := `SELECT * FROM pages WHERE document_id = ? ORDER BY page_number`
	
	err := cm.db.Select(&entries, query, documentID)
	return entries, err
}

// DeleteDocument 删除文档及其所有页面
func (cm *CacheManager) DeleteDocument(documentID string) error {
	tx, err := cm.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除页面
	if _, err := tx.Exec("DELETE FROM pages WHERE document_id = ?", documentID); err != nil {
		return err
	}

	// 删除文档
	if _, err := tx.Exec("DELETE FROM documents WHERE id = ?", documentID); err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupOldCache 清理旧缓存
func (cm *CacheManager) CleanupOldCache(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	
	tx, err := cm.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 获取要删除的文档ID
	var documentIDs []string
	err = tx.Select(&documentIDs, 
		"SELECT id FROM documents WHERE updated_at < ?", cutoff)
	if err != nil {
		return err
	}

	// 删除页面
	for _, docID := range documentIDs {
		if _, err := tx.Exec("DELETE FROM pages WHERE document_id = ?", docID); err != nil {
			return err
		}
	}

	// 删除文档
	if _, err := tx.Exec("DELETE FROM documents WHERE updated_at < ?", cutoff); err != nil {
		return err
	}

	return tx.Commit()
}

// Close 关闭数据库连接
func (cm *CacheManager) Close() error {
	return cm.db.Close()
}
