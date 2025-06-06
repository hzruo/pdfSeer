<script lang="ts" setup>
import { ref, onMounted, watch } from 'vue'
import { GetConfig, UpdateConfig } from '../../wailsjs/go/main/App'

// Emits
const emit = defineEmits<{
  'close': []
}>()

// 响应式数据
const loading = ref(false)
const saving = ref(false)
const config = ref<any>({
  ai: {
    base_url: 'https://api.openai.com/v1',
    api_key: '',
    model: 'gpt-4-vision-preview',
    timeout: 30,
    request_interval: 1.0,
    burst_limit: 3,
    max_retries: 3,
    retry_delay: 1
  },
  storage: {
    cache_ttl: '24h',
    max_cache_size: '2GB',
    history_retention: '30d'
  },
  ui: {
    theme: 'light',
    default_font: 'system',
    layout: 'split'
  }
})

// 模型选项
const modelOptions = ref<Array<{value: string, label: string, description?: string}>>([])
const loadingModels = ref(false)
const modelError = ref('')

// 主题选项
const themeOptions = [
  { value: 'light', label: '浅色主题' },
  { value: 'dark', label: '深色主题' },
  { value: 'auto', label: '跟随系统' }
]

// 生命周期
onMounted(async () => {
  await loadConfig()
})

// 监听API配置变化，自动获取模型列表
watch(() => [config.value.ai.base_url, config.value.ai.api_key],
  async ([newBaseUrl, newApiKey], [oldBaseUrl, oldApiKey]) => {
    if (newBaseUrl && newApiKey &&
        (newBaseUrl !== oldBaseUrl || newApiKey !== oldApiKey)) {
      await fetchModels()
    }
  },
  { deep: true }
)

// 方法
const loadConfig = async () => {
  try {
    loading.value = true
    const currentConfig = await GetConfig()
    if (currentConfig) {
      config.value = currentConfig
      // 如果已有API配置，尝试获取模型列表
      if (config.value.ai.base_url && config.value.ai.api_key) {
        await fetchModels()
      }
    }
  } catch (error) {
    console.error('加载配置失败:', error)
    alert('加载配置失败: ' + error)
  } finally {
    loading.value = false
  }
}

// 获取模型列表
const fetchModels = async () => {
  if (!config.value.ai.base_url || !config.value.ai.api_key) {
    return
  }

  try {
    loadingModels.value = true
    modelError.value = ''

    // 调用OpenAI API获取模型列表
    const response = await fetch(`${config.value.ai.base_url}/models`, {
      headers: {
        'Authorization': `Bearer ${config.value.ai.api_key}`,
        'Content-Type': 'application/json'
      }
    })

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }

    const data = await response.json()

    if (data.data && Array.isArray(data.data)) {
      // 过滤出支持视觉的模型
      const visionModels = data.data.filter((model: any) =>
        model.id.includes('vision') ||
        model.id.includes('gpt-4') ||
        model.id.includes('gpt-4o')
      )

      // 转换为选项格式
      modelOptions.value = visionModels.map((model: any) => ({
        value: model.id,
        label: formatModelName(model.id),
        description: model.description || ''
      }))

      // 如果当前选择的模型不在列表中，选择第一个可用模型
      if (modelOptions.value.length > 0) {
        const currentModel = config.value.ai.model
        const modelExists = modelOptions.value.some(option => option.value === currentModel)
        if (!modelExists) {
          config.value.ai.model = modelOptions.value[0].value
        }
      }
    } else {
      throw new Error('API返回格式不正确')
    }
  } catch (error) {
    console.error('获取模型列表失败:', error)
    modelError.value = `获取模型列表失败: ${error}`

    // 使用默认模型列表
    modelOptions.value = [
      { value: 'gpt-4-vision-preview', label: 'GPT-4 Vision Preview' },
      { value: 'gpt-4-turbo', label: 'GPT-4 Turbo' },
      { value: 'gpt-4o', label: 'GPT-4o' },
      { value: 'gpt-4o-mini', label: 'GPT-4o Mini' }
    ]
  } finally {
    loadingModels.value = false
  }
}

// 格式化模型名称
const formatModelName = (modelId: string) => {
  const nameMap: Record<string, string> = {
    'gpt-4-vision-preview': 'GPT-4 Vision Preview',
    'gpt-4-turbo': 'GPT-4 Turbo',
    'gpt-4o': 'GPT-4o',
    'gpt-4o-mini': 'GPT-4o Mini',
    'gpt-4': 'GPT-4'
  }

  return nameMap[modelId] || modelId
}

const saveConfig = async () => {
  try {
    saving.value = true
    await UpdateConfig(config.value)
    alert('配置保存成功')
  } catch (error) {
    console.error('保存配置失败:', error)
    alert('保存配置失败: ' + error)
  } finally {
    saving.value = false
  }
}

const resetToDefaults = () => {
  if (confirm('确定要重置为默认配置吗？')) {
    config.value = {
      ai: {
        base_url: 'https://api.openai.com/v1',
        api_key: '',
        model: 'gpt-4-vision-preview',
        timeout: 30,
        request_interval: 1.0,
        burst_limit: 3,
        max_retries: 3,
        retry_delay: 1
      },
      storage: {
        cache_ttl: '24h',
        max_cache_size: '2GB',
        history_retention: '30d'
      },
      ui: {
        theme: 'light',
        default_font: 'system',
        layout: 'split'
      }
    }
  }
}

const testConnection = async () => {
  if (!config.value.ai.api_key) {
    alert('请先输入API Key')
    return
  }
  
  // 这里可以添加测试连接的逻辑
  alert('连接测试功能待实现')
}

const close = () => {
  emit('close')
}
</script>

<template>
  <div class="config-overlay">
    <div class="config-panel">
      <!-- 头部 -->
      <div class="panel-header">
        <h2>设置</h2>
        <button @click="close" class="close-btn">×</button>
      </div>

      <!-- 内容 -->
      <div class="panel-content">
        <div v-if="loading" class="loading-state">
          <div class="spinner"></div>
          <p>加载配置中...</p>
        </div>

        <div v-else class="config-sections">
          <!-- AI服务配置 -->
          <section class="config-section">
            <h3>AI服务配置</h3>
            
            <div class="form-group">
              <label for="base-url">API Base URL:</label>
              <input 
                id="base-url"
                v-model="config.ai.base_url" 
                type="url" 
                placeholder="https://api.openai.com/v1"
                class="form-input"
              />
              <small class="form-help">
                支持OpenAI兼容的API服务，如Azure OpenAI、本地部署等
              </small>
            </div>

            <div class="form-group">
              <label for="api-key">API Key:</label>
              <input 
                id="api-key"
                v-model="config.ai.api_key" 
                type="password" 
                placeholder="sk-..."
                class="form-input"
              />
              <small class="form-help">
                您的OpenAI API密钥，将安全存储在本地
              </small>
            </div>

            <div class="form-group">
              <label for="model">AI模型:</label>
              <div class="model-select-container">
                <select
                  id="model"
                  v-model="config.ai.model"
                  class="form-select"
                  :disabled="loadingModels"
                >
                  <option v-if="loadingModels" value="">加载模型列表中...</option>
                  <option v-else-if="modelOptions.length === 0" value="">请先配置API信息</option>
                  <option v-for="option in modelOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
                <button
                  v-if="config.ai.base_url && config.ai.api_key"
                  @click="fetchModels"
                  :disabled="loadingModels"
                  class="refresh-models-btn"
                  title="刷新模型列表"
                >
                  {{ loadingModels ? '⟳' : '🔄' }}
                </button>
              </div>
              <small v-if="modelError" class="form-error">{{ modelError }}</small>
              <small v-else class="form-help">
                自动从API获取可用模型列表，优先显示支持视觉的模型
              </small>
            </div>

            <div class="form-row">
              <div class="form-group">
                <label for="timeout">请求超时(秒):</label>
                <input
                  id="timeout"
                  v-model.number="config.ai.timeout"
                  type="number"
                  min="5"
                  max="300"
                  class="form-input"
                />
              </div>

              <div class="form-group">
                <label for="interval">请求间隔(秒):</label>
                <input
                  id="interval"
                  v-model.number="config.ai.request_interval"
                  type="number"
                  min="0.1"
                  max="10"
                  step="0.1"
                  class="form-input"
                />
              </div>

              <div class="form-group">
                <label for="burst">并发请求数:</label>
                <input
                  id="burst"
                  v-model.number="config.ai.burst_limit"
                  type="number"
                  min="1"
                  max="10"
                  class="form-input"
                />
              </div>
            </div>

            <!-- 重试配置 -->
            <div class="form-row">
              <div class="form-group">
                <label for="max-retries">最大重试次数:</label>
                <input
                  id="max-retries"
                  v-model.number="config.ai.max_retries"
                  type="number"
                  min="0"
                  max="10"
                  class="form-input"
                />
                <small class="form-help">
                  网络错误或API限流时的重试次数，0表示不重试
                </small>
              </div>

              <div class="form-group">
                <label for="retry-delay">重试延迟(秒):</label>
                <input
                  id="retry-delay"
                  v-model.number="config.ai.retry_delay"
                  type="number"
                  min="1"
                  max="30"
                  class="form-input"
                />
                <small class="form-help">
                  重试前的等待时间，实际延迟会根据重试次数递增
                </small>
              </div>
            </div>

            <div class="form-actions">
              <button @click="testConnection" class="btn btn-secondary">
                测试连接
              </button>
            </div>
          </section>

          <!-- 存储配置 -->
          <section class="config-section">
            <h3>存储配置</h3>
            
            <div class="form-row">
              <div class="form-group">
                <label for="cache-ttl">缓存有效期:</label>
                <input 
                  id="cache-ttl"
                  v-model="config.storage.cache_ttl" 
                  type="text" 
                  placeholder="24h"
                  class="form-input"
                />
                <small class="form-help">格式: 24h, 7d, 30d</small>
              </div>

              <div class="form-group">
                <label for="max-cache">最大缓存大小:</label>
                <input 
                  id="max-cache"
                  v-model="config.storage.max_cache_size" 
                  type="text" 
                  placeholder="2GB"
                  class="form-input"
                />
                <small class="form-help">格式: 1GB, 2GB, 5GB</small>
              </div>

              <div class="form-group">
                <label for="history-retention">历史保留期:</label>
                <input 
                  id="history-retention"
                  v-model="config.storage.history_retention" 
                  type="text" 
                  placeholder="30d"
                  class="form-input"
                />
                <small class="form-help">格式: 30d, 90d, 1y</small>
              </div>
            </div>
          </section>

          <!-- 界面配置 -->
          <section class="config-section">
            <h3>界面配置</h3>
            
            <div class="form-row">
              <div class="form-group">
                <label for="theme">主题:</label>
                <select id="theme" v-model="config.ui.theme" class="form-select">
                  <option v-for="option in themeOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
              </div>

              <div class="form-group">
                <label for="font">默认字体:</label>
                <input 
                  id="font"
                  v-model="config.ui.default_font" 
                  type="text" 
                  placeholder="system"
                  class="form-input"
                />
              </div>

              <div class="form-group">
                <label for="layout">布局模式:</label>
                <select id="layout" v-model="config.ui.layout" class="form-select">
                  <option value="split">分栏布局</option>
                  <option value="vertical">垂直布局</option>
                  <option value="horizontal">水平布局</option>
                </select>
              </div>
            </div>
          </section>
        </div>
      </div>

      <!-- 底部操作 -->
      <div class="panel-footer">
        <button @click="resetToDefaults" class="btn btn-outline">
          重置默认
        </button>
        <div class="footer-actions">
          <button @click="close" class="btn btn-secondary">
            取消
          </button>
          <button @click="saveConfig" :disabled="saving" class="btn btn-primary">
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.config-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.config-panel {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15), 0 8px 20px rgba(0, 0, 0, 0.1);
  width: 90%;
  max-width: 800px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: slideIn 0.3s ease;
}

@keyframes slideIn {
  from {
    transform: scale(0.9) translateY(-20px);
    opacity: 0;
  }
  to {
    transform: scale(1) translateY(0);
    opacity: 1;
  }
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.1) 0%, rgba(118, 75, 162, 0.1) 100%);
}

.panel-header h2 {
  margin: 0;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-weight: 600;
  font-size: 1.5rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.panel-header h2::before {
  content: '⚙️';
  font-size: 1.2rem;
}

.close-btn {
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 50%;
  font-size: 1.2rem;
  cursor: pointer;
  color: #666;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 1);
  color: #333;
  transform: scale(1.1);
}

.panel-content {
  flex: 1;
  overflow-y: auto;
  padding: 2rem;
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem;
  color: #666;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #007bff;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 1rem;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.config-sections {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.config-section {
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 12px;
  padding: 2rem;
  background: rgba(255, 255, 255, 0.6);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.05);
  backdrop-filter: blur(10px);
}

.config-section h3 {
  margin: 0 0 1.5rem 0;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 1.2rem;
  font-weight: 600;
  padding-bottom: 0.5rem;
  border-bottom: 2px solid rgba(102, 126, 234, 0.3);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.config-section h3::before {
  font-size: 1rem;
}

.config-section:nth-child(1) h3::before {
  content: '🤖';
}

.config-section:nth-child(2) h3::before {
  content: '💾';
}

.config-section:nth-child(3) h3::before {
  content: '🎨';
}

.form-group {
  margin-bottom: 1rem;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #333;
}

.form-input,
.form-select {
  width: 100%;
  padding: 0.75rem 1rem;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 8px;
  font-size: 0.9rem;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(5px);
  transition: all 0.2s ease;
  font-weight: 500;
}

.form-input:focus,
.form-select:focus {
  outline: none;
  border-color: rgba(102, 126, 234, 0.5);
  box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
  background: rgba(255, 255, 255, 0.95);
  transform: translateY(-1px);
}

.form-help {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.8rem;
  color: #666;
}

.form-error {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.8rem;
  color: #dc3545;
}

.model-select-container {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.model-select-container .form-select {
  flex: 1;
}

.refresh-models-btn {
  background: #f8f9fa;
  border: 1px solid #ccc;
  border-radius: 4px;
  padding: 0.75rem;
  cursor: pointer;
  font-size: 1rem;
  transition: all 0.2s;
  min-width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.refresh-models-btn:hover:not(:disabled) {
  background: #e9ecef;
  border-color: #007bff;
}

.refresh-models-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.form-actions {
  margin-top: 1rem;
}

.panel-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  border-top: 1px solid #e0e0e0;
  background: #f8f9fa;
}

.footer-actions {
  display: flex;
  gap: 1rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 10px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 500;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
}

.btn::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
  transition: left 0.5s;
}

.btn:hover::before {
  left: 100%;
}

.btn-primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.btn-primary:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.4);
}

.btn-primary:disabled {
  background: #ccc;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.btn-secondary {
  background: linear-gradient(135deg, #6c757d 0%, #495057 100%);
  color: white;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.btn-secondary:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(108, 117, 125, 0.4);
}

.btn-outline {
  background: rgba(255, 255, 255, 0.8);
  color: #667eea;
  border: 1px solid rgba(102, 126, 234, 0.3);
  backdrop-filter: blur(5px);
}

.btn-outline:hover {
  background: rgba(102, 126, 234, 0.1);
  border-color: rgba(102, 126, 234, 0.5);
  transform: translateY(-1px);
}
</style>
