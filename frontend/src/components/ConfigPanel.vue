<script lang="ts" setup>
import { ref, onMounted, watch } from 'vue'
import { GetConfig, UpdateConfig, CheckSystemDependencies, GetInstallInstructions } from '../../wailsjs/go/main/App'
import CustomDialog from './CustomDialog.vue'

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

// 依赖状态
const systemDependencies = ref<any>(null)
const loadingDependencies = ref(false)
const installInstructions = ref<any>(null)

// 主题选项
const themeOptions = [
  { value: 'light', label: '浅色主题' },
  { value: 'dark', label: '深色主题' },
  { value: 'auto', label: '跟随系统' }
]

// 对话框状态
const dialog = ref({
  show: false,
  title: '',
  message: '',
  type: 'info' as 'info' | 'success' | 'warning' | 'error' | 'confirm',
  showCancel: false,
  onConfirm: () => {},
  onCancel: () => {}
})

// 生命周期
onMounted(async () => {
  await loadConfig()
  await loadDependencies()
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
    showDialog({
      title: '加载失败',
      message: `加载配置失败: ${error}`,
      type: 'error'
    })
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
      // 不过滤模型，显示所有可用模型
      const allModels = data.data

      // 转换为选项格式
      modelOptions.value = allModels.map((model: any) => ({
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

    // 使用默认模型列表（包含所有类型的模型）
    modelOptions.value = [
      { value: 'gpt-4-vision-preview', label: 'GPT-4 Vision Preview' },
      { value: 'gpt-4-turbo', label: 'GPT-4 Turbo' },
      { value: 'gpt-4o', label: 'GPT-4o' },
      { value: 'gpt-4o-mini', label: 'GPT-4o Mini' },
      { value: 'gpt-4', label: 'GPT-4' },
      { value: 'gpt-3.5-turbo', label: 'GPT-3.5 Turbo' }
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
    'gpt-4': 'GPT-4',
    'gpt-3.5-turbo': 'GPT-3.5 Turbo'
  }

  return nameMap[modelId] || modelId
}

// 检查是否为视觉模型
const isVisionModel = (modelId: string) => {
  const visionModels = [
    'gpt-4-vision-preview',
    'gpt-4-turbo',
    'gpt-4o',
    'gpt-4o-mini'
  ]

  return visionModels.some(vm => modelId.includes(vm))
}

// 对话框辅助函数
const showDialog = (options: {
  title?: string
  message: string
  type?: 'info' | 'success' | 'warning' | 'error' | 'confirm'
  showCancel?: boolean
  onConfirm?: () => void
  onCancel?: () => void
}) => {
  dialog.value = {
    show: true,
    title: options.title || '',
    message: options.message,
    type: options.type || 'info',
    showCancel: options.showCancel || false,
    onConfirm: options.onConfirm || (() => {}),
    onCancel: options.onCancel || (() => {})
  }
}

const hideDialog = () => {
  dialog.value.show = false
}

const saveConfig = async () => {
  try {
    saving.value = true
    await UpdateConfig(config.value)
    showDialog({
      title: '保存成功',
      message: '配置已成功保存',
      type: 'success'
    })
  } catch (error) {
    console.error('保存配置失败:', error)
    showDialog({
      title: '保存失败',
      message: `保存配置失败: ${error}`,
      type: 'error'
    })
  } finally {
    saving.value = false
  }
}

const resetToDefaults = () => {
  showDialog({
    title: '重置配置',
    message: '确定要重置为默认配置吗？此操作将清除所有当前设置。',
    type: 'confirm',
    showCancel: true,
    onConfirm: () => {
      config.value = {
        ai: {
          base_url: 'https://api.openai.com/v1',
          api_key: '',
          model: 'gpt-4-vision-preview',
          ocr_model: 'gpt-4-vision-preview',
          text_model: 'gpt-4',
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
      showDialog({
        title: '重置成功',
        message: '配置已重置为默认值',
        type: 'success'
      })
    }
  })
}

const testConnection = async () => {
  if (!config.value.ai.api_key) {
    showDialog({
      title: '配置不完整',
      message: '请先输入API Key',
      type: 'warning'
    })
    return
  }

  if (!config.value.ai.base_url) {
    showDialog({
      title: '配置不完整',
      message: '请先输入API Base URL',
      type: 'warning'
    })
    return
  }

  try {
    // 测试连接
    const response = await fetch(`${config.value.ai.base_url}/models`, {
      headers: {
        'Authorization': `Bearer ${config.value.ai.api_key}`,
        'Content-Type': 'application/json'
      }
    })

    if (response.ok) {
      showDialog({
        title: '连接成功',
        message: 'API连接测试成功，可以正常使用',
        type: 'success'
      })
    } else {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
  } catch (error) {
    console.error('连接测试失败:', error)
    showDialog({
      title: '连接失败',
      message: `连接测试失败: ${error}`,
      type: 'error'
    })
  }
}

// 加载依赖状态
const loadDependencies = async () => {
  try {
    loadingDependencies.value = true

    // 检查系统依赖
    const deps = await CheckSystemDependencies()
    systemDependencies.value = deps

    // 获取安装说明
    const instructions = await GetInstallInstructions()
    installInstructions.value = instructions

    console.log('依赖检查结果:', deps)
    console.log('安装说明:', instructions)
  } catch (error) {
    console.error('检查依赖失败:', error)
    showDialog({
      title: '检查失败',
      message: `检查系统依赖失败: ${error}`,
      type: 'error'
    })
  } finally {
    loadingDependencies.value = false
  }
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

            <!-- OCR识别模型 -->
            <div class="form-group">
              <label for="ocr-model">OCR识别模型:</label>
              <div class="model-select-container">
                <select
                  id="ocr-model"
                  v-model="config.ai.ocr_model"
                  class="form-select"
                  :disabled="loadingModels"
                >
                  <option v-if="loadingModels" value="">加载模型列表中...</option>
                  <option v-else-if="modelOptions.length === 0" value="">请先配置API信息</option>
                  <option v-for="option in modelOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                    <span v-if="isVisionModel(option.value)" class="model-badge">📷 视觉</span>
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
                用于图片OCR识别，建议选择支持视觉的模型（如GPT-4 Vision）
              </small>
            </div>

            <!-- 文本处理模型 -->
            <div class="form-group">
              <label for="text-model">文本处理模型:</label>
              <select
                id="text-model"
                v-model="config.ai.text_model"
                class="form-select"
                :disabled="loadingModels"
              >
                <option v-if="loadingModels" value="">加载模型列表中...</option>
                <option v-else-if="modelOptions.length === 0" value="">请先配置API信息</option>
                <option v-for="option in modelOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                  <span v-if="!isVisionModel(option.value)" class="model-badge">💬 文本</span>
                </option>
              </select>
              <small class="form-help">
                用于AI文本处理（纠错、总结、翻译等），可选择文本专用模型以降低成本
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

          <!-- 系统依赖状态 -->
          <section class="config-section">
            <h3>系统依赖状态</h3>

            <div v-if="loadingDependencies" class="loading-state">
              <div class="spinner"></div>
              <p>检查系统依赖中...</p>
            </div>

            <div v-else-if="systemDependencies" class="dependency-status">
              <div class="system-info">
                <p><strong>系统信息:</strong> {{ systemDependencies.os }}/{{ systemDependencies.arch }}</p>
              </div>

              <div class="dependency-list">
                <div v-for="dep in systemDependencies.dependencies" :key="dep.name" class="dependency-item">
                  <div class="dependency-header">
                    <span class="dependency-icon">{{ dep.installed ? '✅' : '❌' }}</span>
                    <span class="dependency-name">{{ dep.name }}</span>
                    <span v-if="dep.required" class="required-badge">必需</span>
                    <span v-else class="optional-badge">可选</span>
                  </div>

                  <div class="dependency-details">
                    <div v-if="dep.version" class="dependency-version">
                      版本: {{ dep.version }}
                    </div>
                    <div class="dependency-description">
                      {{ dep.description }}
                    </div>
                    <div v-if="dep.error" class="dependency-error">
                      {{ dep.error }}
                    </div>

                    <!-- 安装说明 -->
                    <div v-if="!dep.installed && installInstructions && installInstructions[dep.name]" class="install-instructions">
                      <details>
                        <summary>安装说明</summary>
                        <pre>{{ installInstructions[dep.name] }}</pre>
                      </details>
                    </div>
                  </div>
                </div>
              </div>

              <div class="dependency-actions">
                <button @click="loadDependencies" :disabled="loadingDependencies" class="btn btn-secondary">
                  {{ loadingDependencies ? '检查中...' : '重新检查' }}
                </button>
              </div>
            </div>

            <div v-else class="no-data">
              <p>无法获取系统依赖信息</p>
              <button @click="loadDependencies" class="btn btn-secondary">
                重试
              </button>
            </div>
          </section>

          <!-- 界面配置 (暂未实现) -->
          <!--
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
          -->
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

    <!-- 自定义对话框 -->
    <CustomDialog
      :show="dialog.show"
      :title="dialog.title"
      :message="dialog.message"
      :type="dialog.type"
      :show-cancel="dialog.showCancel"
      @confirm="dialog.onConfirm"
      @cancel="dialog.onCancel"
      @close="hideDialog"
    />
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
  content: '🔧';
}

.config-section:nth-child(4) h3::before {
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

.model-badge {
  font-size: 0.7rem;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  margin-left: 0.5rem;
  font-weight: 500;
  background: rgba(0, 123, 255, 0.1);
  color: #007bff;
}

/* 依赖状态样式 */
.dependency-status {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.system-info {
  padding: 0.75rem;
  background: rgba(102, 126, 234, 0.1);
  border-radius: 8px;
  border: 1px solid rgba(102, 126, 234, 0.2);
}

.system-info p {
  margin: 0;
  color: #333;
  font-size: 0.9rem;
}

.dependency-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.dependency-item {
  padding: 1rem;
  background: rgba(255, 255, 255, 0.8);
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  transition: all 0.2s ease;
}

.dependency-item:hover {
  background: rgba(255, 255, 255, 0.95);
  border-color: rgba(102, 126, 234, 0.3);
  transform: translateY(-1px);
}

.dependency-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}

.dependency-icon {
  font-size: 1.1rem;
}

.dependency-name {
  font-weight: 600;
  color: #333;
  flex: 1;
  font-size: 0.95rem;
}

.required-badge {
  background: rgba(220, 53, 69, 0.1);
  color: #dc3545;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}

.optional-badge {
  background: rgba(108, 117, 125, 0.1);
  color: #6c757d;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}

.dependency-details {
  margin-left: 1.85rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.dependency-version {
  color: #28a745;
  font-size: 0.85rem;
  font-weight: 500;
}

.dependency-description {
  color: #666;
  font-size: 0.85rem;
  line-height: 1.4;
}

.dependency-error {
  color: #dc3545;
  font-size: 0.85rem;
  font-weight: 500;
}

.install-instructions {
  margin-top: 0.5rem;
}

.install-instructions details {
  background: rgba(248, 249, 250, 0.8);
  border-radius: 6px;
  padding: 0.5rem;
}

.install-instructions summary {
  cursor: pointer;
  font-weight: 500;
  color: #007bff;
  font-size: 0.85rem;
  padding: 0.25rem;
}

.install-instructions summary:hover {
  color: #0056b3;
}

.install-instructions pre {
  margin: 0.5rem 0 0 0;
  padding: 0.75rem;
  background: rgba(33, 37, 41, 0.95);
  color: #f8f9fa;
  border-radius: 4px;
  font-size: 0.8rem;
  line-height: 1.4;
  overflow-x: auto;
  white-space: pre-wrap;
}

.dependency-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.5rem;
}

.no-data {
  text-align: center;
  padding: 2rem;
  color: #666;
}

.no-data p {
  margin-bottom: 1rem;
}
</style>
