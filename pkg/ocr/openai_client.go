package ocr

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"pdf-ocr-ai/pkg/config"
	"pdf-ocr-ai/pkg/ratelimiter"
)

// OpenAIClient OpenAI客户端
type OpenAIClient struct {
	client      *openai.Client
	config      config.AIConfig
	rateLimiter *ratelimiter.RateLimiter
}

// OCRResult OCR识别结果
type OCRResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Error      string  `json:"error,omitempty"`
}

// NewOpenAIClient 创建OpenAI客户端
func NewOpenAIClient(cfg config.AIConfig) *OpenAIClient {
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)
	
	// 创建频率限制器
	rateLimiter := ratelimiter.NewRateLimiter(cfg.RequestInterval, cfg.BurstLimit)

	return &OpenAIClient{
		client:      client,
		config:      cfg,
		rateLimiter: rateLimiter,
	}
}

// RecognizeImage 识别图片中的文字
func (c *OpenAIClient) RecognizeImage(ctx context.Context, imagePath string) (*OCRResult, error) {
	// 等待频率限制
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("频率限制等待失败: %w", err)
	}

	// 读取图片文件
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("读取图片失败: %w", err)
	}

	// 转换为base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	// 获取OCR专用模型，如果没有配置则使用默认模型
	ocrModel := c.config.OCRModel
	if ocrModel == "" {
		ocrModel = c.config.Model
	}

	// 根据模型类型构建不同的请求
	if c.isVisionModel(ocrModel) {
		return c.recognizeWithVision(timeoutCtx, base64Image, ocrModel)
	} else {
		return c.recognizeWithText(timeoutCtx, imagePath, ocrModel)
	}
}

// isVisionModel 检查是否为视觉模型 - 使用更宽松的检测策略
func (c *OpenAIClient) isVisionModel(model string) bool {
	if model == "" {
		return false
	}

	lowerModel := strings.ToLower(model)

	// 明确不支持视觉的模型
	textOnlyModels := []string{
		"gpt-3.5-turbo",
		"gpt-3.5",
		"text-davinci",
		"text-curie",
		"text-babbage",
		"text-ada",
	}

	// 检查是否为明确的文本模型
	for _, textModel := range textOnlyModels {
		if strings.Contains(lowerModel, textModel) {
			return false
		}
	}

	// 对于其他模型，默认假设支持视觉功能
	// 这样可以避免误判新的视觉模型（如Gemini、Claude等）
	// 如果模型实际不支持视觉，API会返回相应错误
	return true
}

// recognizeWithVision 使用视觉模型识别
func (c *OpenAIClient) recognizeWithVision(ctx context.Context, base64Image string, model string) (*OCRResult, error) {
	// 构建请求
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `你是一个专业的OCR识别引擎。请精确识别图片中的所有文字，要求：
1. 保持原始排版格式和换行
2. 如果包含表格，请用Markdown格式输出
3. 如果是扫描文档，请修正常见的OCR错误
4. 直接返回识别的文字内容，不要使用代码块格式，不要添加任何解释或说明
5. 如果无法识别任何文字，返回空字符串`,
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: "请识别这张图片中的所有文字内容：",
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    fmt.Sprintf("data:image/jpeg;base64,%s", base64Image),
							Detail: openai.ImageURLDetailHigh,
						},
					},
				},
			},
		},
		MaxTokens:   4000,
		Temperature: 0.1, // 低温度确保一致性
	}

	// 发送请求（带重试机制）
	var resp openai.ChatCompletionResponse
	retryConfig := c.getRetryConfig()
	err := retryWithBackoff(ctx, retryConfig, func() error {
		var apiErr error
		resp, apiErr = c.client.CreateChatCompletion(ctx, req)
		return apiErr
	})

	if err != nil {
		return &OCRResult{
			Error: fmt.Sprintf("API调用失败: %v", err),
		}, err
	}

	if len(resp.Choices) == 0 {
		return &OCRResult{
			Error: "未收到AI响应",
		}, fmt.Errorf("未收到AI响应")
	}

	// 清理结果文本，移除可能的代码块格式
	text := strings.TrimSpace(resp.Choices[0].Message.Content)
	text = cleanOCRResult(text)

	result := &OCRResult{
		Text:       text,
		Confidence: 0.95, // OpenAI通常有较高的准确率
	}

	return result, nil
}

// recognizeWithText 使用文本模型识别（需要先用其他OCR引擎）
func (c *OpenAIClient) recognizeWithText(ctx context.Context, imagePath string, model string) (*OCRResult, error) {
	// 对于非视觉模型，返回提示信息
	return &OCRResult{
		Text:       "此模型不支持图片识别，请使用支持视觉的模型如 gpt-4-vision-preview",
		Confidence: 0.0,
		Error:      "模型不支持视觉功能",
	}, fmt.Errorf("模型 %s 不支持图片识别", model)
}

// GetSupportedModels 获取支持的模型列表
func (c *OpenAIClient) GetSupportedModels() []ModelInfo {
	return []ModelInfo{
		{
			ID:          "gpt-4-vision-preview",
			Name:        "GPT-4 Vision Preview",
			Description: "GPT-4的视觉预览版本，支持图片和文本处理",
			SupportsVision: true,
			MaxTokens:   4096,
			Recommended: true,
		},
		{
			ID:          "gpt-4-turbo",
			Name:        "GPT-4 Turbo",
			Description: "GPT-4的高速版本，支持视觉功能",
			SupportsVision: true,
			MaxTokens:   4096,
			Recommended: true,
		},
		{
			ID:          "gpt-4o",
			Name:        "GPT-4o",
			Description: "GPT-4的优化版本，多模态支持",
			SupportsVision: true,
			MaxTokens:   4096,
			Recommended: true,
		},
		{
			ID:          "gpt-4o-mini",
			Name:        "GPT-4o Mini",
			Description: "GPT-4o的轻量版本，成本更低",
			SupportsVision: true,
			MaxTokens:   4096,
			Recommended: false,
		},
		{
			ID:          "gpt-4",
			Name:        "GPT-4",
			Description: "标准GPT-4模型，仅支持文本",
			SupportsVision: false,
			MaxTokens:   4096,
			Recommended: false,
		},
		{
			ID:          "gpt-3.5-turbo",
			Name:        "GPT-3.5 Turbo",
			Description: "GPT-3.5的高速版本，仅支持文本",
			SupportsVision: false,
			MaxTokens:   4096,
			Recommended: false,
		},
	}
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	SupportsVision bool   `json:"supports_vision"`
	MaxTokens      int    `json:"max_tokens"`
	Recommended    bool   `json:"recommended"`
}

// RecognizeImageFromReader 从Reader识别图片
func (c *OpenAIClient) RecognizeImageFromReader(ctx context.Context, reader io.Reader) (*OCRResult, error) {
	// 读取数据
	imageData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取图片数据失败: %w", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "ocr_*.jpg")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 写入数据
	if _, err := tmpFile.Write(imageData); err != nil {
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 调用识别
	return c.RecognizeImage(ctx, tmpFile.Name())
}

// ProcessWithAI 使用AI处理文本（纠错、总结等）
func (c *OpenAIClient) ProcessWithAI(ctx context.Context, text string, prompt string) (string, error) {
	// 等待频率限制
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("频率限制等待失败: %w", err)
	}

	// 创建超时上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	// 获取文本处理专用模型，如果没有配置则使用默认模型
	textModel := c.config.TextModel
	if textModel == "" {
		textModel = c.config.Model
	}
	if textModel == "" {
		textModel = "gpt-4" // 最后的备选方案
	}

	// 构建请求
	req := openai.ChatCompletionRequest{
		Model: textModel, // 使用配置的文本处理模型
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: prompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.3,
	}

	// 发送请求（带重试机制）
	var resp openai.ChatCompletionResponse
	retryConfig := c.getRetryConfig()
	err := retryWithBackoff(timeoutCtx, retryConfig, func() error {
		var apiErr error
		resp, apiErr = c.client.CreateChatCompletion(timeoutCtx, req)
		return apiErr
	})

	if err != nil {
		return "", fmt.Errorf("AI处理失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("未收到AI响应")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// UpdateConfig 更新配置
func (c *OpenAIClient) UpdateConfig(cfg config.AIConfig) {
	c.config = cfg
	
	// 更新客户端配置
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}
	c.client = openai.NewClientWithConfig(clientConfig)
	
	// 更新频率限制器
	c.rateLimiter.UpdateRate(cfg.RequestInterval, cfg.BurstLimit)
}

// Close 关闭客户端
func (c *OpenAIClient) Close() {
	if c.rateLimiter != nil {
		c.rateLimiter.Close()
	}
}

// getRetryConfig 获取重试配置
func (c *OpenAIClient) getRetryConfig() RetryConfig {
	config := DefaultRetryConfig

	// 使用配置中的重试参数
	if c.config.MaxRetries > 0 {
		config.MaxRetries = c.config.MaxRetries
	}

	if c.config.RetryDelay > 0 {
		config.BaseDelay = time.Duration(c.config.RetryDelay) * time.Second
	}

	return config
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int           // 最大重试次数
	BaseDelay  time.Duration // 基础延迟时间
	MaxDelay   time.Duration // 最大延迟时间
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	BaseDelay:  1 * time.Second,
	MaxDelay:   10 * time.Second,
}

// isRetryableError 判断是否为可重试的错误
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// 网络相关错误
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "reset") {
		return true
	}

	// API限流错误
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "429") {
		return true
	}

	// 服务器错误
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "gateway timeout") {
		return true
	}

	return false
}

// calculateBackoffDelay 计算退避延迟时间（指数退避）
func calculateBackoffDelay(attempt int, config RetryConfig) time.Duration {
	if attempt <= 0 {
		return config.BaseDelay
	}

	// 指数退避：baseDelay * 2^attempt
	delay := config.BaseDelay * time.Duration(1<<uint(attempt))

	// 限制最大延迟时间
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// retryWithBackoff 带退避的重试函数
func retryWithBackoff(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// 执行操作
		err := operation()
		if err == nil {
			// 成功，返回
			return nil
		}

		lastErr = err

		// 如果是最后一次尝试，直接返回错误
		if attempt == config.MaxRetries {
			break
		}

		// 检查是否为可重试的错误
		if !isRetryableError(err) {
			log.Printf("遇到不可重试的错误，停止重试: %v", err)
			return err
		}

		// 计算延迟时间
		delay := calculateBackoffDelay(attempt, config)
		log.Printf("第 %d 次重试失败: %v，%v 后重试", attempt+1, err, delay)

		// 等待延迟时间
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 继续下一次重试
		}
	}

	log.Printf("重试 %d 次后仍然失败，最后错误: %v", config.MaxRetries, lastErr)
	return fmt.Errorf("重试 %d 次后仍然失败: %w", config.MaxRetries, lastErr)
}

// cleanOCRResult 清理OCR识别结果，移除不必要的格式
func cleanOCRResult(text string) string {
	// 移除开头和结尾的代码块标记
	text = strings.TrimSpace(text)

	// 移除开头的 ```
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 1 {
			// 移除第一行（```或```语言标识）
			text = strings.Join(lines[1:], "\n")
		}
	}

	// 移除结尾的 ```
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}

	// 移除其他常见的格式标记
	text = strings.TrimSpace(text)

	// 移除可能的语言标识行（如果第一行只包含语言名称）
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		firstLine := strings.TrimSpace(lines[0])
		// 如果第一行是常见的语言标识，移除它
		if firstLine == "text" || firstLine == "markdown" || firstLine == "md" ||
		   firstLine == "txt" || firstLine == "plain" || len(firstLine) < 10 {
			text = strings.Join(lines[1:], "\n")
		}
	}

	return strings.TrimSpace(text)
}
