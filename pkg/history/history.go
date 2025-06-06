package history

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// ProcessingStatus 处理状态
type ProcessingStatus string

const (
	StatusProcessing ProcessingStatus = "processing"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
)

// HistoryRecord 历史记录
type HistoryRecord struct {
	ID           int              `db:"id" json:"id"`
	DocumentPath string           `db:"document_path" json:"document_path"`
	DocumentName string           `db:"document_name" json:"document_name"`
	PageCount    int              `db:"page_count" json:"page_count"`
	Status       ProcessingStatus `db:"status" json:"status"`
	AIModel      string           `db:"ai_model" json:"ai_model"`
	Cost         float64          `db:"cost" json:"cost"`
	ProcessedAt  string           `db:"processed_at" json:"processed_at"`
	CompletedAt  *string          `db:"completed_at" json:"completed_at,omitempty"`
	ErrorMessage *string          `db:"error_message" json:"error_message,omitempty"`
}

// HistoryPage 历史页面
type HistoryPage struct {
	ID               int     `db:"id" json:"id"`
	HistoryID        int     `db:"history_id" json:"history_id"`
	PageNumber       int     `db:"page_number" json:"page_number"`
	OriginalText     string  `db:"original_text" json:"original_text"`
	OCRText          string  `db:"ocr_text" json:"ocr_text"`
	AIProcessedText  string  `db:"ai_processed_text" json:"ai_processed_text"`
	ProcessingTime   float64 `db:"processing_time" json:"processing_time"` // 处理时间（秒）
	CreatedAt        string  `db:"created_at" json:"created_at"`
}

// SearchResult 搜索结果
type SearchResult struct {
	HistoryID    int    `json:"history_id"`
	DocumentPath string `json:"document_path"`
	DocumentName string `json:"document_name"`
	PageNumber   int    `json:"page_number"`
	Snippet      string `json:"snippet"`
	ProcessedAt  string `json:"processed_at"`
}

// HistoryManager 历史记录管理器
type HistoryManager struct {
	db        *sqlx.DB
	ftsEnabled bool // 是否支持FTS5
}

// NewHistoryManager 创建历史记录管理器
func NewHistoryManager() (*HistoryManager, error) {
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
	dbPath := filepath.Join(dataDir, "history.db")
	db, err := sqlx.Connect("sqlite3", dbPath+"?cache=shared&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	hm := &HistoryManager{db: db}

	// 检测FTS5支持
	hm.ftsEnabled = hm.checkFTS5Support()

	// 初始化数据库表
	if err := hm.initTables(); err != nil {
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	return hm, nil
}

// checkFTS5Support 检测FTS5支持
func (hm *HistoryManager) checkFTS5Support() bool {
	// 尝试创建一个临时的FTS5表来检测支持
	_, err := hm.db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS fts_test USING fts5(content)")
	if err != nil {
		return false
	}

	// 清理测试表
	hm.db.Exec("DROP TABLE IF EXISTS fts_test")
	return true
}

// initTables 初始化数据库表
func (hm *HistoryManager) initTables() error {
	// 历史记录表
	historySQL := `
	CREATE TABLE IF NOT EXISTS processing_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		document_path TEXT NOT NULL,
		document_name TEXT NOT NULL,
		page_count INTEGER NOT NULL,
		status TEXT CHECK(status IN ('processing', 'completed', 'failed')) DEFAULT 'processing',
		ai_model TEXT,
		cost REAL DEFAULT 0,
		processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		error_message TEXT
	);`

	// 历史页面表
	pagesSQL := `
	CREATE TABLE IF NOT EXISTS history_pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		history_id INTEGER NOT NULL,
		page_number INTEGER NOT NULL,
		original_text TEXT,
		ocr_text TEXT,
		ai_processed_text TEXT,
		processing_time REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (history_id) REFERENCES processing_history(id),
		UNIQUE(history_id, page_number)
	);`

	// 创建索引
	indexSQL := `
	CREATE INDEX IF NOT EXISTS idx_history_status ON processing_history(status);
	CREATE INDEX IF NOT EXISTS idx_history_date ON processing_history(processed_at);
	CREATE INDEX IF NOT EXISTS idx_pages_history ON history_pages(history_id);
	`

	// 执行基础SQL
	for _, sql := range []string{historySQL, pagesSQL, indexSQL} {
		if _, err := hm.db.Exec(sql); err != nil {
			return fmt.Errorf("执行SQL失败: %w", err)
		}
	}

	// 如果支持FTS5，创建全文搜索表
	if hm.ftsEnabled {
		ftsSQL := `
		CREATE VIRTUAL TABLE IF NOT EXISTS history_search USING fts5(
			history_id,
			document_path,
			document_name,
			ocr_text,
			ai_processed_text,
			content='history_pages',
			content_rowid='id'
		);`

		if _, err := hm.db.Exec(ftsSQL); err != nil {
			// 如果FTS5表创建失败，禁用FTS功能但不返回错误
			hm.ftsEnabled = false
		}
	}

	return nil
}

// CreateRecord 创建历史记录
func (hm *HistoryManager) CreateRecord(documentPath string, pageCount int, aiModel string) (*HistoryRecord, error) {
	documentName := filepath.Base(documentPath)
	
	query := `
	INSERT INTO processing_history (document_path, document_name, page_count, ai_model)
	VALUES (?, ?, ?, ?)
	`
	
	result, err := hm.db.Exec(query, documentPath, documentName, pageCount, aiModel)
	if err != nil {
		return nil, fmt.Errorf("创建历史记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取记录ID失败: %w", err)
	}

	return hm.GetRecord(int(id))
}

// GetRecord 获取历史记录
func (hm *HistoryManager) GetRecord(id int) (*HistoryRecord, error) {
	var record HistoryRecord
	query := `SELECT * FROM processing_history WHERE id = ?`
	
	err := hm.db.Get(&record, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	return &record, err
}

// UpdateRecordStatus 更新记录状态
func (hm *HistoryManager) UpdateRecordStatus(id int, status ProcessingStatus, errorMsg string) error {
	query := `
	UPDATE processing_history
	SET status = ?, error_message = ?, completed_at = CASE WHEN ? = 'completed' THEN CURRENT_TIMESTAMP ELSE completed_at END
	WHERE id = ?
	`

	var errorMsgPtr *string
	if errorMsg != "" {
		errorMsgPtr = &errorMsg
	}

	_, err := hm.db.Exec(query, status, errorMsgPtr, status, id)
	return err
}

// AddPage 添加页面记录
func (hm *HistoryManager) AddPage(page *HistoryPage) error {
	query := `
	INSERT OR REPLACE INTO history_pages 
	(history_id, page_number, original_text, ocr_text, ai_processed_text, processing_time)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	
	_, err := hm.db.Exec(query, page.HistoryID, page.PageNumber, 
		page.OriginalText, page.OCRText, page.AIProcessedText, page.ProcessingTime)
	
	if err != nil {
		return err
	}

	// 更新全文搜索索引（如果支持FTS5）
	if hm.ftsEnabled {
		return hm.updateSearchIndex(page.HistoryID, page.PageNumber)
	}

	return nil
}

// updateSearchIndex 更新搜索索引
func (hm *HistoryManager) updateSearchIndex(historyID int, pageNumber int) error {
	query := `
	INSERT OR REPLACE INTO history_search 
	SELECT 
		hp.id,
		ph.id,
		ph.document_path,
		ph.document_name,
		hp.ocr_text,
		hp.ai_processed_text
	FROM history_pages hp
	JOIN processing_history ph ON hp.history_id = ph.id
	WHERE hp.history_id = ? AND hp.page_number = ?
	`
	
	_, err := hm.db.Exec(query, historyID, pageNumber)
	return err
}

// GetRecentRecords 获取最近的记录
func (hm *HistoryManager) GetRecentRecords(limit int) ([]*HistoryRecord, error) {
	var records []*HistoryRecord
	query := `
	SELECT * FROM processing_history 
	ORDER BY processed_at DESC 
	LIMIT ?
	`
	
	err := hm.db.Select(&records, query, limit)
	return records, err
}

// GetRecordPages 获取记录的所有页面
func (hm *HistoryManager) GetRecordPages(historyID int) ([]*HistoryPage, error) {
	var pages []*HistoryPage
	query := `
	SELECT * FROM history_pages
	WHERE history_id = ?
	ORDER BY page_number
	`

	err := hm.db.Select(&pages, query, historyID)
	return pages, err
}

// GetDocumentPages 获取文档所有历史记录的页面数据
func (hm *HistoryManager) GetDocumentPages(documentPath string) ([]*HistoryPage, error) {
	var pages []*HistoryPage
	query := `
	SELECT hp.* FROM history_pages hp
	JOIN processing_history ph ON hp.history_id = ph.id
	WHERE ph.document_path = ?
	ORDER BY hp.page_number
	`

	err := hm.db.Select(&pages, query, documentPath)
	return pages, err
}

// GetRecordsByDocumentPath 获取指定文档路径的所有历史记录
func (hm *HistoryManager) GetRecordsByDocumentPath(documentPath string) ([]*HistoryRecord, error) {
	var records []*HistoryRecord
	query := `
	SELECT * FROM processing_history
	WHERE document_path = ?
	ORDER BY processed_at DESC
	`

	err := hm.db.Select(&records, query, documentPath)
	return records, err
}

// GetPages 获取记录的所有页面（别名方法，保持兼容性）
func (hm *HistoryManager) GetPages(historyID int) ([]*HistoryPage, error) {
	return hm.GetRecordPages(historyID)
}

// SearchContent 搜索内容
func (hm *HistoryManager) SearchContent(keyword string, limit int) ([]*SearchResult, error) {
	var results []*SearchResult

	if hm.ftsEnabled {
		// 使用FTS5搜索
		query := `
		SELECT
			hs.history_id,
			hs.document_path,
			hs.document_name,
			hp.page_number,
			snippet(history_search, 3, '<mark>', '</mark>', '...', 32) as snippet,
			ph.processed_at
		FROM history_search hs
		JOIN history_pages hp ON hs.rowid = hp.id
		JOIN processing_history ph ON hs.history_id = ph.id
		WHERE history_search MATCH ?
		ORDER BY rank
		LIMIT ?
		`

		err := hm.db.Select(&results, query, keyword, limit)
		return results, err
	} else {
		// 使用普通LIKE搜索作为后备方案
		query := `
		SELECT
			ph.id as history_id,
			ph.document_path,
			ph.document_name,
			hp.page_number,
			CASE
				WHEN hp.ocr_text LIKE ? THEN substr(hp.ocr_text, 1, 100) || '...'
				WHEN hp.ai_processed_text LIKE ? THEN substr(hp.ai_processed_text, 1, 100) || '...'
				ELSE '...'
			END as snippet,
			ph.processed_at
		FROM history_pages hp
		JOIN processing_history ph ON hp.history_id = ph.id
		WHERE hp.ocr_text LIKE ? OR hp.ai_processed_text LIKE ?
		ORDER BY ph.processed_at DESC
		LIMIT ?
		`

		searchPattern := "%" + keyword + "%"
		err := hm.db.Select(&results, query, searchPattern, searchPattern, searchPattern, searchPattern, limit)
		return results, err
	}
}

// DeleteRecord 删除记录
func (hm *HistoryManager) DeleteRecord(id int) error {
	tx, err := hm.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除搜索索引（如果支持FTS5）
	if hm.ftsEnabled {
		if _, err := tx.Exec("DELETE FROM history_search WHERE history_id = ?", id); err != nil {
			return err
		}
	}

	// 删除页面
	if _, err := tx.Exec("DELETE FROM history_pages WHERE history_id = ?", id); err != nil {
		return err
	}

	// 删除记录
	if _, err := tx.Exec("DELETE FROM processing_history WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupOldRecords 清理旧记录
func (hm *HistoryManager) CleanupOldRecords(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	// 获取要删除的记录ID
	var recordIDs []int
	err := hm.db.Select(&recordIDs,
		"SELECT id FROM processing_history WHERE processed_at < ?", cutoff)
	if err != nil {
		return err
	}

	// 删除每个记录
	for _, id := range recordIDs {
		if err := hm.DeleteRecord(id); err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭数据库连接
func (hm *HistoryManager) Close() error {
	return hm.db.Close()
}
