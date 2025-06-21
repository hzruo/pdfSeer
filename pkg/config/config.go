package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// AIConfig AI服务配置
type AIConfig struct {
	BaseURL         string  `json:"base_url"`
	APIKey          string  `json:"api_key"`
	Model           string  `json:"model"`           // 保持向后兼容，默认OCR模型
	OCRModel        string  `json:"ocr_model"`       // OCR识别专用模型
	TextModel       string  `json:"text_model"`      // 文本处理专用模型
	ModelsEndpoint  string  `json:"models_endpoint"` // 模型列表API端点
	ChatEndpoint    string  `json:"chat_endpoint"`   // 对话API端点
	Timeout         int     `json:"timeout"`
	RequestInterval float64 `json:"request_interval"`
	BurstLimit      int     `json:"burst_limit"`
	MaxRetries      int     `json:"max_retries"` // 最大重试次数
	RetryDelay      int     `json:"retry_delay"` // 重试延迟（秒）
}

// StorageConfig 存储配置
type StorageConfig struct {
	CacheTTL         string `json:"cache_ttl"`
	MaxCacheSize     string `json:"max_cache_size"`
	HistoryRetention string `json:"history_retention"`
}

// UIConfig 界面配置
type UIConfig struct {
	Theme       string `json:"theme"`
	DefaultFont string `json:"default_font"`
	Layout      string `json:"layout"`
}

// AppConfig 应用配置
type AppConfig struct {
	AI      AIConfig      `json:"ai"`
	Storage StorageConfig `json:"storage"`
	UI      UIConfig      `json:"ui"`
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     AppConfig
	mu         sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	configDir := filepath.Join(homeDir, ".pdfSeer")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")

	cm := &ConfigManager{
		configPath: configPath,
	}

	// 加载配置
	if err := cm.Load(); err != nil {
		return nil, err
	}

	return cm, nil
}

// Load 加载配置
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 设置默认配置
	cm.config = AppConfig{
		AI: AIConfig{
			BaseURL:         "https://api.openai.com/v1",
			Model:           "gpt-4-vision-preview", // 保持向后兼容
			OCRModel:        "gpt-4-vision-preview", // OCR默认使用视觉模型
			TextModel:       "gpt-4",                // 文本处理默认使用GPT-4
			ModelsEndpoint:  "/models",              // 默认模型列表端点
			ChatEndpoint:    "/chat/completions",    // 默认对话端点
			Timeout:         30,
			RequestInterval: 1.0,
			BurstLimit:      3,
			MaxRetries:      3, // 默认重试3次
			RetryDelay:      1, // 默认延迟1秒
		},
		Storage: StorageConfig{
			CacheTTL:         "24h",
			MaxCacheSize:     "2GB",
			HistoryRetention: "30d",
		},
		UI: UIConfig{
			Theme:       "light",
			DefaultFont: "system",
			Layout:      "split",
		},
	}

	// 尝试从文件加载
	if data, err := os.ReadFile(cm.configPath); err == nil {
		if err := json.Unmarshal(data, &cm.config); err != nil {
			return fmt.Errorf("解析配置文件失败: %w", err)
		}
	}

	return nil
}

// Save 保存配置
func (cm *ConfigManager) Save() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0600); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}

	return nil
}

// GetAIConfig 获取AI配置
func (cm *ConfigManager) GetAIConfig() AIConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config.AI
}

// UpdateAIConfig 更新AI配置
func (cm *ConfigManager) UpdateAIConfig(config AIConfig) error {
	cm.mu.Lock()
	cm.config.AI = config
	cm.mu.Unlock()

	return cm.Save()
}

// GetConfig 获取完整配置
func (cm *ConfigManager) GetConfig() AppConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// UpdateConfig 更新完整配置
func (cm *ConfigManager) UpdateConfig(config AppConfig) error {
	cm.mu.Lock()
	cm.config = config
	cm.mu.Unlock()

	return cm.Save()
}
