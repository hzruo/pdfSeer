<script lang="ts" setup>
import { ref, onMounted, watch, nextTick } from 'vue'
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
    models_endpoint: '/models',
    chat_endpoint: '/chat/completions',
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
const dependenciesLoaded = ref(false)  // 标记依赖是否已加载

// 当前选择的预设
const selectedPreset = ref('')

// 保存用户的自定义配置
const customConfig = ref({
  base_url: '',
  models_endpoint: '/models',
  chat_endpoint: '/chat/completions',
  api_key: '',
  ocr_model: '',
  text_model: ''
})

// 标记是否已经有真正的自定义配置
const hasRealCustomConfig = ref(false)

// 自定义配置管理
const savedConfigs = ref<Array<{
  id: string
  name: string
  base_url: string
  models_endpoint: string
  chat_endpoint: string
  api_key: string
  ocr_model: string
  text_model: string
  created_at: string
}>>([])

// 保存配置对话框状态
const showSaveConfigDialog = ref(false)
const configName = ref('')
const savingConfig = ref(false)

// 配置管理对话框状态
const showConfigManagerDialog = ref(false)

// 删除配置确认对话框
const showDeleteConfirm = ref(false)
const configToDelete = ref('')

// 主题选项
const themeOptions = [
  { value: 'light', label: '浅色主题' },
  { value: 'dark', label: '深色主题' },
  { value: 'auto', label: '跟随系统' }
]

// API服务预设模板
const apiPresets = [
  {
    name: 'OpenAI',
    base_url: 'https://api.openai.com/v1',
    models_endpoint: '/models',
    chat_endpoint: '/chat/completions'
  },
  {
    name: 'Google Gemini',
    base_url: 'https://generativelanguage.googleapis.com/v1beta/openai',
    models_endpoint: '/models',
    chat_endpoint: '/chat/completions'
  },
  {
    name: 'Pollinations（免费）',
    base_url: 'https://text.pollinations.ai/openai',
    models_endpoint: '/models',
    chat_endpoint: '/chat/completions',
    api_key: 'sk-pollination',
    ocr_model: 'openai-large',
    text_model: 'deepseek-reasoning'
  },
  {
    name: '自定义配置',
    base_url: '',
    models_endpoint: '/models',
    chat_endpoint: '/chat/completions'
  }
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
  // 优先加载配置，不等待依赖检测
  await loadConfig()

  // 加载保存的自定义配置
  loadSavedConfigs()

  // 异步加载依赖状态，不阻塞页面显示
  setTimeout(() => {
    loadDependencies()
  }, 200)
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

// 标记是否正在应用预设，避免循环触发
const applyingPreset = ref(false)

// 监听配置变化，自动检测预设
watch(() => [config.value.ai.base_url, config.value.ai.models_endpoint, config.value.ai.chat_endpoint, config.value.ai.api_key, config.value.ai.ocr_model, config.value.ai.text_model],
  () => {
    // 如果正在应用预设，跳过检测
    if (applyingPreset.value) {
      return
    }

    // 如果当前是自定义配置模式，且用户有输入，标记为真实的自定义配置
    if (selectedPreset.value === '自定义配置' && config.value.ai.base_url) {
      hasRealCustomConfig.value = true
      saveAsCustomConfig()
      console.log('用户在自定义配置模式下输入，保存配置:', customConfig.value)
    }

    // 只有在不是通过预设选择器触发的变化时才重新检测
    setTimeout(() => {
      const oldPreset = selectedPreset.value
      detectCurrentPreset()

      // 如果从预设变为自定义配置，说明用户手动修改了配置
      if (oldPreset !== '自定义配置' && oldPreset !== '' && selectedPreset.value === '自定义配置') {
        console.log('用户手动修改配置，自动保存为自定义配置')
        hasRealCustomConfig.value = true
        saveAsCustomConfig()
      }
    }, 50)
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

      // 检测当前配置对应的预设
      detectCurrentPreset()

      // 如果已有API配置，异步获取模型列表（不阻塞页面加载）
      if (config.value.ai.base_url && config.value.ai.api_key) {
        // 使用setTimeout让模型获取异步进行
        setTimeout(() => {
          fetchModels()
        }, 100)
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

    // 构建模型API URL
    const modelsEndpoint = config.value.ai.models_endpoint || '/models'
    const modelsUrl = `${config.value.ai.base_url}${modelsEndpoint}`

    console.log('获取模型列表:', modelsUrl)

    // 调用API获取模型列表
    const response = await fetch(modelsUrl, {
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

    // 清空模型列表，不显示默认模型
    modelOptions.value = []
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

// 移除视觉模型检测 - 让用户自己判断，避免误导

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
          models_endpoint: '/models',
          chat_endpoint: '/chat/completions',
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
    // 构建测试URL
    const modelsEndpoint = config.value.ai.models_endpoint || '/models'
    const testUrl = `${config.value.ai.base_url}${modelsEndpoint}`

    console.log('测试连接:', testUrl)

    // 测试连接
    const response = await fetch(testUrl, {
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
  // 如果已经加载过，直接返回
  if (dependenciesLoaded.value) {
    return
  }

  try {
    loadingDependencies.value = true

    // 检查系统依赖
    const deps = await CheckSystemDependencies()
    systemDependencies.value = deps

    // 获取安装说明
    const instructions = await GetInstallInstructions()
    installInstructions.value = instructions

    // 标记为已加载
    dependenciesLoaded.value = true

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

// 强制重新加载依赖（用于手动重新检查）
const forceReloadDependencies = async () => {
  dependenciesLoaded.value = false
  await loadDependencies()
}

// 检测当前配置对应的预设
const detectCurrentPreset = () => {
  const currentConfig = config.value.ai

  for (const preset of apiPresets) {
    // 跳过自定义配置预设
    if (preset.name === '自定义配置') continue

    // 基本URL和端点匹配
    const baseUrlMatch = preset.base_url === currentConfig.base_url
    const modelsEndpointMatch = preset.models_endpoint === (currentConfig.models_endpoint || '/models')
    const chatEndpointMatch = preset.chat_endpoint === (currentConfig.chat_endpoint || '/chat/completions')

    if (baseUrlMatch && modelsEndpointMatch && chatEndpointMatch) {
      // 对于Pollinations预设，还需要检查模型配置
      if (preset.name === 'Pollinations（免费）') {
        const ocrModelMatch = preset.ocr_model === currentConfig.ocr_model
        const textModelMatch = preset.text_model === currentConfig.text_model

        if (ocrModelMatch && textModelMatch) {
          selectedPreset.value = preset.name
          return
        }
      } else {
        selectedPreset.value = preset.name
        return
      }
    }
  }

  // 如果没有匹配的预设，设置为自定义
  if (currentConfig.base_url) {
    selectedPreset.value = '自定义配置'
    // 不在这里保存，让用户手动输入时再保存
    console.log('检测到自定义配置，但不自动保存')
  } else {
    selectedPreset.value = ''
  }
}

// 保存当前配置为自定义配置
const saveAsCustomConfig = () => {
  customConfig.value = {
    base_url: config.value.ai.base_url,
    models_endpoint: config.value.ai.models_endpoint || '/models',
    chat_endpoint: config.value.ai.chat_endpoint || '/chat/completions',
    api_key: config.value.ai.api_key,
    ocr_model: config.value.ai.ocr_model,
    text_model: config.value.ai.text_model
  }
}

// 应用预设配置
const applyPreset = (event: Event) => {
  const target = event.target as HTMLSelectElement
  const presetName = target.value

  // 标记正在应用预设，避免触发watch
  applyingPreset.value = true

  try {
    // 只有当前真的是自定义配置且有真实配置时才保存
    if (selectedPreset.value === '自定义配置' && hasRealCustomConfig.value) {
      saveAsCustomConfig()
      console.log('保存当前自定义配置:', customConfig.value)
    }

    selectedPreset.value = presetName

    if (!presetName) return

    // 检查是否是自定义保存的配置
    if (presetName.startsWith('custom_')) {
      const configId = presetName.replace('custom_', '')
      loadCustomConfig(configId)
      return
    }

    if (presetName === '自定义配置') {
      // 立即重置标记，确保界面能正常更新
      applyingPreset.value = false

      // 如果有真实的自定义配置，恢复它
      if (hasRealCustomConfig.value) {
        config.value.ai.base_url = customConfig.value.base_url
        config.value.ai.models_endpoint = customConfig.value.models_endpoint
        config.value.ai.chat_endpoint = customConfig.value.chat_endpoint
        config.value.ai.api_key = customConfig.value.api_key
        config.value.ai.ocr_model = customConfig.value.ocr_model
        config.value.ai.text_model = customConfig.value.text_model

        console.log('恢复自定义配置:', customConfig.value)
      } else {
        // 如果没有真实的自定义配置，清空所有字段
        config.value.ai.base_url = ''
        config.value.ai.models_endpoint = '/models'
        config.value.ai.chat_endpoint = '/chat/completions'
        config.value.ai.api_key = ''
        config.value.ai.ocr_model = ''
        config.value.ai.text_model = ''

        console.log('初始化空的自定义配置')
      }

      // 清空模型列表和错误信息
      modelOptions.value = []
      modelError.value = ''

      // 强制触发响应式更新
      nextTick(() => {
        // 强制重新渲染表单元素
        const baseUrlInput = document.getElementById('base-url') as HTMLInputElement
        const apiKeyInput = document.getElementById('api-key') as HTMLInputElement
        const ocrModelSelect = document.getElementById('ocr-model') as HTMLSelectElement
        const textModelSelect = document.getElementById('text-model') as HTMLSelectElement

        if (baseUrlInput) baseUrlInput.value = config.value.ai.base_url
        if (apiKeyInput) apiKeyInput.value = config.value.ai.api_key
        if (ocrModelSelect) ocrModelSelect.value = config.value.ai.ocr_model
        if (textModelSelect) textModelSelect.value = config.value.ai.text_model

        // 如果有API Key和Base URL，自动获取模型列表
        if (config.value.ai.api_key && config.value.ai.base_url) {
          setTimeout(() => {
            fetchModels()
          }, 100)
        }
      })
      return
    }

    const preset = apiPresets.find(p => p.name === presetName)
    if (preset) {
      config.value.ai.base_url = preset.base_url
      config.value.ai.models_endpoint = preset.models_endpoint
      config.value.ai.chat_endpoint = preset.chat_endpoint

      // 清空API Key（除非预设自带API Key）
      if (preset.api_key) {
        config.value.ai.api_key = preset.api_key
      } else {
        config.value.ai.api_key = ''
      }

      // 清空模型选择（除非预设指定默认模型）
      if (preset.ocr_model) {
        config.value.ai.ocr_model = preset.ocr_model
      } else {
        config.value.ai.ocr_model = ''
      }

      if (preset.text_model) {
        config.value.ai.text_model = preset.text_model
      } else {
        config.value.ai.text_model = ''
      }

      // 清空模型列表和错误信息
      modelOptions.value = []
      modelError.value = ''

      console.log('应用预设配置:', preset)

      // 强制更新界面
      nextTick(() => {
        // 强制重新渲染表单元素
        const baseUrlInput = document.getElementById('base-url') as HTMLInputElement
        const apiKeyInput = document.getElementById('api-key') as HTMLInputElement
        const ocrModelSelect = document.getElementById('ocr-model') as HTMLSelectElement
        const textModelSelect = document.getElementById('text-model') as HTMLSelectElement

        if (baseUrlInput) baseUrlInput.value = config.value.ai.base_url
        if (apiKeyInput) apiKeyInput.value = config.value.ai.api_key
        if (ocrModelSelect) ocrModelSelect.value = config.value.ai.ocr_model
        if (textModelSelect) textModelSelect.value = config.value.ai.text_model

        // 如果有API Key和Base URL，自动获取模型列表
        if (config.value.ai.api_key && preset.base_url) {
          setTimeout(() => {
            fetchModels()
          }, 100)
        }
      })
    }
  } finally {
    // 延迟重置标记，确保配置更新完成
    setTimeout(() => {
      applyingPreset.value = false
    }, 200)
  }
}

// 自定义配置管理方法
const loadSavedConfigs = () => {
  try {
    const saved = localStorage.getItem('ai_custom_configs')
    if (saved) {
      savedConfigs.value = JSON.parse(saved)
    }
  } catch (error) {
    console.error('加载自定义配置失败:', error)
  }
}

const saveSavedConfigs = () => {
  try {
    localStorage.setItem('ai_custom_configs', JSON.stringify(savedConfigs.value))
  } catch (error) {
    console.error('保存自定义配置失败:', error)
  }
}

const openSaveConfigDialog = () => {
  if (!config.value.ai.base_url) {
    showDialog({
      title: '配置不完整',
      message: '请先配置API Base URL',
      type: 'warning'
    })
    return
  }

  configName.value = ''
  showSaveConfigDialog.value = true
}

const openConfigManagerDialog = () => {
  showConfigManagerDialog.value = true
}

const saveCustomConfig = async () => {
  if (!configName.value.trim()) {
    showDialog({
      title: '配置名称不能为空',
      message: '请输入配置名称',
      type: 'warning'
    })
    return
  }

  // 检查名称是否已存在
  const existingConfig = savedConfigs.value.find(c => c.name === configName.value.trim())
  if (existingConfig) {
    showDialog({
      title: '配置名称已存在',
      message: '请使用不同的配置名称',
      type: 'warning'
    })
    return
  }

  try {
    savingConfig.value = true

    const newConfig = {
      id: Date.now().toString(),
      name: configName.value.trim(),
      base_url: config.value.ai.base_url,
      models_endpoint: config.value.ai.models_endpoint || '/models',
      chat_endpoint: config.value.ai.chat_endpoint || '/chat/completions',
      api_key: config.value.ai.api_key,
      ocr_model: config.value.ai.ocr_model || '',
      text_model: config.value.ai.text_model || '',
      created_at: new Date().toISOString()
    }

    savedConfigs.value.push(newConfig)
    saveSavedConfigs()

    showSaveConfigDialog.value = false
    configName.value = ''

    showDialog({
      title: '保存成功',
      message: `配置"${newConfig.name}"已保存`,
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
    savingConfig.value = false
  }
}

const loadCustomConfig = (configId: string) => {
  const savedConfig = savedConfigs.value.find(c => c.id === configId)
  if (!savedConfig) return

  // 标记正在应用预设，避免触发watch
  applyingPreset.value = true

  try {
    config.value.ai.base_url = savedConfig.base_url
    config.value.ai.models_endpoint = savedConfig.models_endpoint
    config.value.ai.chat_endpoint = savedConfig.chat_endpoint
    config.value.ai.api_key = savedConfig.api_key
    config.value.ai.ocr_model = savedConfig.ocr_model
    config.value.ai.text_model = savedConfig.text_model

    selectedPreset.value = `custom_${configId}`

    // 清空模型列表和错误信息
    modelOptions.value = []
    modelError.value = ''

    console.log('加载自定义配置:', savedConfig)

    // 如果有API Key和Base URL，自动获取模型列表
    if (savedConfig.api_key && savedConfig.base_url) {
      setTimeout(() => {
        fetchModels()
      }, 100)
    }
  } finally {
    setTimeout(() => {
      applyingPreset.value = false
    }, 200)
  }
}

const confirmDeleteConfig = (configId: string) => {
  const configToDeleteObj = savedConfigs.value.find(c => c.id === configId)
  if (!configToDeleteObj) return

  showDialog({
    title: '删除配置',
    message: `确定要删除配置"${configToDeleteObj.name}"吗？此操作不可撤销。`,
    type: 'confirm',
    showCancel: true,
    onConfirm: () => deleteCustomConfig(configId)
  })
}

const deleteCustomConfig = (configId: string) => {
  const index = savedConfigs.value.findIndex(c => c.id === configId)
  if (index === -1) return

  const deletedConfig = savedConfigs.value[index]
  savedConfigs.value.splice(index, 1)
  saveSavedConfigs()

  // 如果当前选择的是被删除的配置，重置选择
  if (selectedPreset.value === `custom_${configId}`) {
    selectedPreset.value = ''
  }

  showDialog({
    title: '删除成功',
    message: `配置"${deletedConfig.name}"已删除`,
    type: 'success'
  })
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

            <!-- 预设模板选择 -->
            <div class="form-group">
              <label for="api-preset">API服务预设:</label>
              <div class="preset-container">
                <select
                  id="api-preset"
                  v-model="selectedPreset"
                  @change="applyPreset"
                  class="form-select"
                >
                  <option value="">选择预设模板...</option>

                  <!-- 内置预设 -->
                  <optgroup label="内置预设">
                    <option v-for="preset in apiPresets" :key="preset.name" :value="preset.name">
                      {{ preset.name }}
                    </option>
                  </optgroup>

                  <!-- 自定义配置 -->
                  <optgroup v-if="savedConfigs.length > 0" label="我的配置">
                    <option v-for="config in savedConfigs" :key="config.id" :value="`custom_${config.id}`">
                      {{ config.name }}
                    </option>
                  </optgroup>
                </select>

                <div class="preset-actions">
                  <button
                    @click="openSaveConfigDialog"
                    class="action-btn save-btn"
                    :title="config.ai.base_url ? '保存当前配置为自定义预设' : '请先配置API信息后再保存'"
                    :disabled="!config.ai.base_url"
                  >
                    💾
                  </button>

                  <button
                    v-if="savedConfigs.length > 0"
                    @click="openConfigManagerDialog"
                    class="action-btn manage-btn"
                    title="管理我的配置"
                  >
                    ⚙️
                  </button>
                </div>
              </div>
              <small class="form-help">
                选择常用的API服务预设，💾保存当前配置，⚙️管理已保存的配置
              </small>
            </div>

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
                API服务的基础URL，不包含具体的端点路径
              </small>
            </div>

            <!-- 端点配置 -->
            <div class="form-row">
              <div class="form-group">
                <label for="models-endpoint">模型列表端点:</label>
                <input
                  id="models-endpoint"
                  v-model="config.ai.models_endpoint"
                  type="text"
                  placeholder="/models"
                  class="form-input"
                />
                <small class="form-help">获取模型列表的API端点</small>
              </div>

              <div class="form-group">
                <label for="chat-endpoint">对话端点:</label>
                <input
                  id="chat-endpoint"
                  v-model="config.ai.chat_endpoint"
                  type="text"
                  placeholder="/chat/completions"
                  class="form-input"
                />
                <small class="form-help">发送对话请求的API端点</small>
              </div>
            </div>

            <!-- URL预览 -->
            <div class="url-preview">
              <div class="preview-item">
                <strong>模型列表URL:</strong>
                <code>{{ config.ai.base_url }}{{ config.ai.models_endpoint || '/models' }}</code>
              </div>
              <div class="preview-item">
                <strong>对话API URL:</strong>
                <code>{{ config.ai.base_url }}{{ config.ai.chat_endpoint || '/chat/completions' }}</code>
              </div>
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
                用于图片OCR识别，请选择支持视觉功能的模型
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
                </option>
              </select>
              <small class="form-help">
                用于AI文本处理（纠错、总结、翻译等）
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
                <button @click="forceReloadDependencies" :disabled="loadingDependencies" class="btn btn-secondary">
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

    <!-- 保存配置对话框 -->
    <div v-if="showSaveConfigDialog" class="dialog-overlay" @click="showSaveConfigDialog = false">
      <div class="dialog-content save-config-dialog" @click.stop>
        <div class="dialog-header">
          <h4>💾 保存配置</h4>
          <button @click="showSaveConfigDialog = false" class="close-btn">×</button>
        </div>

        <div class="dialog-body">
          <div class="form-group">
            <label for="config-name">配置名称:</label>
            <input
              id="config-name"
              v-model="configName"
              type="text"
              placeholder="例如：我的OpenAI配置"
              class="form-input"
              @keyup.enter="saveCustomConfig"
              autofocus
            />
            <small class="form-help">
              为此配置起一个便于识别的名称
            </small>
          </div>

          <div class="config-preview">
            <h5>配置预览:</h5>
            <div class="preview-item">
              <strong>Base URL:</strong> {{ config.ai.base_url }}
            </div>
            <div class="preview-item">
              <strong>OCR模型:</strong> {{ config.ai.ocr_model || '未设置' }}
            </div>
            <div class="preview-item">
              <strong>文本模型:</strong> {{ config.ai.text_model || '未设置' }}
            </div>
          </div>
        </div>

        <div class="dialog-footer">
          <button @click="showSaveConfigDialog = false" class="btn btn-secondary">取消</button>
          <button
            @click="saveCustomConfig"
            :disabled="!configName.trim() || savingConfig"
            class="btn btn-primary"
          >
            {{ savingConfig ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 配置管理对话框 -->
    <div v-if="showConfigManagerDialog" class="dialog-overlay" @click="showConfigManagerDialog = false">
      <div class="dialog-content config-manager-dialog" @click.stop>
        <div class="dialog-header">
          <h4>⚙️ 管理我的配置</h4>
          <button @click="showConfigManagerDialog = false" class="close-btn">×</button>
        </div>

        <div class="dialog-body">
          <div v-if="savedConfigs.length === 0" class="empty-state">
            <div class="empty-icon">📋</div>
            <p>还没有保存的配置</p>
            <small>配置好API信息后，点击💾按钮保存配置</small>
          </div>

          <div v-else class="custom-configs-list">
            <div v-for="savedConfig in savedConfigs" :key="savedConfig.id" class="custom-config-item">
              <div class="config-info">
                <div class="config-name">{{ savedConfig.name }}</div>
                <div class="config-details">
                  <span class="config-url">{{ savedConfig.base_url }}</span>
                  <span class="config-date">{{ new Date(savedConfig.created_at).toLocaleDateString() }}</span>
                </div>
                <div class="config-models">
                  <span v-if="savedConfig.ocr_model" class="model-tag">OCR: {{ savedConfig.ocr_model }}</span>
                  <span v-if="savedConfig.text_model" class="model-tag">文本: {{ savedConfig.text_model }}</span>
                </div>
              </div>
              <div class="config-actions">
                <button
                  @click="loadCustomConfig(savedConfig.id); showConfigManagerDialog = false"
                  class="btn-small btn-primary"
                  title="加载此配置"
                >
                  加载
                </button>
                <button
                  @click="confirmDeleteConfig(savedConfig.id)"
                  class="btn-small btn-danger"
                  title="删除此配置"
                >
                  删除
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="dialog-footer">
          <div class="footer-info">
            <small>共 {{ savedConfigs.length }} 个配置</small>
          </div>
          <button @click="showConfigManagerDialog = false" class="btn btn-secondary">关闭</button>
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
  position: relative;
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

/* URL预览样式 */
.url-preview {
  margin-top: 1rem;
  padding: 1rem;
  background: rgba(248, 249, 250, 0.8);
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
}

.preview-item {
  margin-bottom: 0.5rem;
  font-size: 0.85rem;
}

.preview-item:last-child {
  margin-bottom: 0;
}

.preview-item strong {
  color: #333;
  margin-right: 0.5rem;
}

.preview-item code {
  background: rgba(33, 37, 41, 0.1);
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.8rem;
  color: #495057;
  word-break: break-all;
}

/* 预设容器样式 */
.preset-container {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.preset-container .form-select {
  flex: 1;
}

.preset-actions {
  display: flex;
  gap: 0.5rem;
}

.action-btn {
  background: rgba(102, 126, 234, 0.1);
  border: 1px solid rgba(102, 126, 234, 0.3);
  border-radius: 8px;
  padding: 0.75rem;
  cursor: pointer;
  font-size: 1rem;
  transition: all 0.2s ease;
  min-width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #667eea;
  position: relative;
}

.action-btn:hover:not(:disabled) {
  background: rgba(102, 126, 234, 0.2);
  border-color: rgba(102, 126, 234, 0.5);
  transform: translateY(-1px);
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none;
}

.save-btn:hover:not(:disabled) {
  background: rgba(40, 167, 69, 0.1);
  border-color: rgba(40, 167, 69, 0.3);
  color: #28a745;
}

.manage-btn:hover:not(:disabled) {
  background: rgba(255, 193, 7, 0.1);
  border-color: rgba(255, 193, 7, 0.3);
  color: #ffc107;
}

/* 配置管理对话框样式 */
.config-manager-dialog {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  width: 90%;
  max-width: 700px;
  max-height: 80vh;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: slideIn 0.3s ease;
  display: flex;
  flex-direction: column;
}

.config-manager-dialog .dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.1) 0%, rgba(118, 75, 162, 0.1) 100%);
  border-radius: 12px 12px 0 0;
}

.config-manager-dialog .dialog-header h4 {
  margin: 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
}

.config-manager-dialog .dialog-body {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
  min-height: 0;
  max-height: 60vh;
}

.config-manager-dialog .dialog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  background: rgba(248, 249, 250, 0.8);
  border-radius: 0 0 12px 12px;
}

.config-manager-dialog .close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #666;
  padding: 0.25rem;
  border-radius: 4px;
  transition: all 0.2s ease;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.config-manager-dialog .close-btn:hover {
  background: rgba(0, 0, 0, 0.1);
  color: #333;
}

.empty-state {
  text-align: center;
  padding: 3rem 2rem;
  color: #666;
}

.empty-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.5;
}

.empty-state p {
  margin: 0 0 0.5rem 0;
  font-size: 1.1rem;
  font-weight: 500;
}

.empty-state small {
  color: #999;
  font-size: 0.9rem;
}

.custom-configs-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.custom-config-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 1rem;
  background: rgba(255, 255, 255, 0.8);
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  transition: all 0.2s ease;
  margin-bottom: 0.75rem;
}

.custom-config-item:hover {
  background: rgba(255, 255, 255, 0.95);
  border-color: rgba(102, 126, 234, 0.3);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.config-info {
  flex: 1;
  margin-right: 1rem;
}

.config-name {
  font-weight: 600;
  color: #333;
  font-size: 1rem;
  margin-bottom: 0.5rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.config-name::before {
  content: '🔧';
  font-size: 0.9rem;
}

.config-details {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.85rem;
  color: #666;
  margin-bottom: 0.5rem;
}

.config-url {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background: rgba(102, 126, 234, 0.1);
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  color: #667eea;
  font-weight: 500;
  display: inline-block;
}

.config-date {
  color: #999;
  font-size: 0.8rem;
}

.config-models {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.model-tag {
  background: rgba(40, 167, 69, 0.1);
  color: #28a745;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  border: 1px solid rgba(40, 167, 69, 0.2);
}

.config-actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  align-items: flex-end;
}

.btn-small {
  padding: 0.5rem 1rem;
  font-size: 0.85rem;
  border-radius: 8px;
  border: none;
  cursor: pointer;
  font-weight: 500;
  transition: all 0.2s ease;
  min-width: 60px;
  white-space: nowrap;
}

.btn-small.btn-primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.2);
}

.btn-small.btn-primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

.btn-small.btn-danger {
  background: linear-gradient(135deg, #dc3545 0%, #c82333 100%);
  color: white;
  box-shadow: 0 2px 8px rgba(220, 53, 69, 0.2);
}

.btn-small.btn-danger:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(220, 53, 69, 0.4);
}

/* 保存配置对话框样式 */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
  animation: fadeIn 0.3s ease;
}

.save-config-dialog {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  width: 90%;
  max-width: 500px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: slideIn 0.3s ease;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.dialog-header h4 {
  margin: 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
}

.dialog-body {
  padding: 1.5rem;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  padding: 1rem 1.5rem;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  background: rgba(248, 249, 250, 0.8);
}

.config-preview {
  margin-top: 1rem;
  padding: 1rem;
  background: rgba(248, 249, 250, 0.8);
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
}

.config-preview h5 {
  margin: 0 0 0.75rem 0;
  color: #333;
  font-size: 0.9rem;
  font-weight: 600;
}

/* 对话框底部信息样式 */
.footer-info {
  flex: 1;
  display: flex;
  align-items: center;
}

.footer-info small {
  color: #666;
  font-size: 0.85rem;
}

/* 响应式优化 */
@media (max-width: 768px) {
  .config-manager-dialog {
    width: 95%;
    max-width: none;
    margin: 1rem;
  }

  .custom-config-item {
    flex-direction: column;
    align-items: stretch;
  }

  .config-info {
    margin-right: 0;
    margin-bottom: 1rem;
  }

  .config-actions {
    flex-direction: row;
    justify-content: flex-end;
  }

  .preset-actions {
    flex-direction: column;
  }

  .action-btn {
    min-width: 40px;
    height: 40px;
  }
}
</style>
