<script lang="ts" setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { SelectFile, GetPageImage, GetPDFPath, ExtractNativeText, ProcessWithAI } from '../../wailsjs/go/main/App'
import { renderMarkdown } from '../utils/markdown'

// Props
interface Props {
  document: any
  selectedPages: number[]
  supportedFormats?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  supportedFormats: () => ['.pdf']
})

// Emits
const emit = defineEmits<{
  'file-select': [filePath: string]
  'page-select': [pageNumbers: number[]]
  'edit-page': [pageNumber: number]
  'process-pages': [pageNumbers: number[], forceReprocess?: boolean]
  'page-rendered': [pageNumber: number]
  'ai-processing-complete': [data: { pages: number[], result: string }]
}>()

// 响应式数据
const currentPage = ref(1)
const pageImages = ref<Map<number, string>>(new Map())
const loading = ref(false)
const viewMode = ref<'single' | 'grid'>('single')
const pdfPath = ref('')
const gridSize = ref(200) // 网格图片大小
const activeTab = ref('ocr') // 当前激活的结果标签页
const imageScale = ref(1) // 图片缩放比例
const splitPosition = ref(50) // 分栏位置百分比
const isDragging = ref(false) // 是否正在拖拽分割线
const isRefreshing = ref(false) // 是否正在刷新文档数据，避免无限循环
const extractingNativeText = ref(false) // 是否正在提取原生文本

// AI处理相关状态
const showAIPromptDialog = ref(false) // 是否显示AI提示词对话框
const processingAI = ref(false) // 是否正在进行AI处理
const aiPrompt = ref('') // AI处理提示词
const aiProcessingMessage = ref('正在连接AI服务...') // AI处理状态消息

// AI提示词预设
const promptPresets = [
  {
    name: '纠错',
    prompt: '请纠正以下文本中的OCR识别错误，保持原有格式和结构，只修正错误的字符和单词：',
    description: '纠正OCR识别中的错误字符'
  },
  {
    name: '总结',
    prompt: '请总结以下文本的主要内容，提取关键信息和要点：',
    description: '总结文本的主要内容'
  },
  {
    name: '翻译',
    prompt: '请将以下文本翻译为英文，保持原有的格式和结构：',
    description: '翻译为英文'
  },
  {
    name: '格式化',
    prompt: '请将以下文本格式化为清晰的Markdown格式，包括适当的标题、段落和列表：',
    description: '格式化为Markdown'
  },
  {
    name: '提取',
    prompt: '请从以下文本中提取关键信息，包括重要的数据、日期、人名、地名等：',
    description: '提取关键信息'
  },
  {
    name: '解答',
    prompt: '请根据以下题目要求完成作答：\n核心内容总结： 清晰、准确地概括文本的核心信息或主旨。\n分步骤解析：\n展示思考过程： 根据题目难度，清晰展示你的关键推理步骤和分析路径（例如：识别关键信息、建立联系、排除干扰项、应用概念/公式/规则等）。\n语言类题目专项： 如涉及语言（词汇、语法、句法、语义、修辞等），必须详细讲解相关要点（例如：解释关键词含义、分析句子结构/成分、说明语法规则应用、阐述表达效果等）。\n复杂学科题目辅助： 如题目涉及复杂逻辑、空间关系、抽象概念（如数学、物理、化学、生物、地理等），必要时可结合示意图、流程图、图表等进行辅助讲解，以增强理解。\n表达规范： 语言简洁清晰，逻辑连贯，术语准确，避免口语化。',
    description: '根据题目要求进行详细解答分析'
  }
]

// 图片模态对话框状态
const showImageModal = ref(false)

// 计算属性
const hasDocument = computed(() => props.document && props.document.pages)
const totalPages = computed(() => props.document?.page_count || 0)
const currentPageData = computed(() => {
  if (!hasDocument.value || currentPage.value < 1) return null
  return props.document.pages[currentPage.value - 1]
})

// AI处理结果的markdown渲染
const renderedAIText = computed(() => {
  if (!currentPageData.value?.ai_text) {
    return ''
  }

  console.log('原始AI文本:', currentPageData.value.ai_text)
  const rendered = renderMarkdown(currentPageData.value.ai_text)
  console.log('渲染后的HTML:', rendered)
  return rendered
})

// 监听文档变化
watch(() => props.document, async (newDoc, oldDoc) => {
  if (newDoc) {
    // 如果是同一个文档的刷新（路径相同），保持当前页面
    const isSameDocument = oldDoc && newDoc.file_path === oldDoc.file_path

    if (isSameDocument) {
      console.log('同一文档刷新，保持当前页面:', currentPage.value)
      // 设置刷新标志，避免触发无限循环
      isRefreshing.value = true
      // 只刷新当前页面的图片，不重置页面状态，跳过事件触发
      await loadPageImage(currentPage.value, true, true)
      // 重置刷新标志
      setTimeout(() => {
        isRefreshing.value = false
      }, 100)
    } else {
      console.log('文档变化，重置状态')
      currentPage.value = 1
      pageImages.value.clear()

      // 立即加载第一页图片
      console.log('开始加载第一页图片')
      await loadPageImage(1)
    }
  }
}, { immediate: true })

// 监听文档刷新事件（保持当前页面）
const handleDocumentRefreshed = (event: any) => {
  const { keepCurrentPage, processedPages } = event.detail
  if (keepCurrentPage) {
    console.log('收到文档刷新事件，保持当前页面:', currentPage.value)
    // 设置刷新标志
    isRefreshing.value = true
    // 强制重新加载当前页面图片以显示最新的处理结果，跳过事件触发
    loadPageImage(currentPage.value, true, true)

    // 如果有处理的页面信息，也预加载这些页面
    if (processedPages && processedPages.length > 0) {
      processedPages.forEach((pageNum: number) => {
        if (pageNum !== currentPage.value) {
          loadPageImage(pageNum, true, true)
        }
      })
    }

    // 重置刷新标志
    setTimeout(() => {
      isRefreshing.value = false
    }, 100)
  }
}

// 添加事件监听器
if (typeof window !== 'undefined') {
  window.addEventListener('document-refreshed', handleDocumentRefreshed)
}

// 方法
const selectFile = async () => {
  try {
    const filePath = await SelectFile()
    if (filePath) {
      emit('file-select', filePath)
    }
  } catch (error) {
    console.error('选择文件失败:', error)
  }
}

const loadPageImage = async (pageNum: number, forceReload = false, skipEvent = false) => {
  if (!hasDocument.value) {
    console.log('没有文档，跳过图片加载')
    return
  }

  if (!forceReload && pageImages.value.has(pageNum)) {
    console.log(`第${pageNum}页图片已存在，跳过加载`)
    return
  }

  try {
    loading.value = true
    console.log(`开始加载第${pageNum}页图片...${forceReload ? '(强制重新加载)' : ''}`)

    const imageData = await GetPageImage(pageNum)
    console.log(`获取到图片数据类型:`, typeof imageData)
    console.log(`获取到图片数据长度:`, imageData ? imageData.length : 'null')

    if (imageData && imageData.length > 0) {
      // Wails 自动将 []byte 转换为 base64 字符串
      if (typeof imageData === 'string') {
        // 直接使用 base64 字符串
        const imageUrl = `data:image/jpeg;base64,${imageData}`
        pageImages.value.set(pageNum, imageUrl)
        console.log(`第${pageNum}页图片加载成功，URL 长度: ${imageUrl.length}`)
        console.log(`pageImages Map 大小: ${pageImages.value.size}`)
        console.log(`base64 前缀:`, (imageData as string).slice(0, 50))

        // 强制触发 Vue 响应式更新
        pageImages.value = new Map(pageImages.value)

        // 只有在非刷新状态下才通知父组件页面已渲染
        if (!skipEvent && !isRefreshing.value) {
          console.log(`通知父组件页面 ${pageNum} 已渲染`)
          emit('page-rendered', pageNum)
        } else {
          console.log(`跳过页面渲染事件，skipEvent: ${skipEvent}, isRefreshing: ${isRefreshing.value}`)
        }
      } else {
        console.error('意外的图片数据格式:', typeof imageData)
      }
    } else {
      console.error(`第${pageNum}页图片数据为空或无效`)
    }
  } catch (error) {
    console.error(`加载第${pageNum}页图片失败:`, error)
  } finally {
    loading.value = false
  }
}

const goToPage = (pageNum: number) => {
  if (pageNum >= 1 && pageNum <= totalPages.value) {
    console.log(`切换到第${pageNum}页，当前页: ${currentPage.value}`)
    currentPage.value = pageNum

    // 强制加载当前页图片（即使已缓存）
    console.log(`强制加载第${pageNum}页图片`)
    loadPageImage(pageNum, true) // 强制重新加载

    // 确保 Vue 响应式更新
    nextTick(() => {
      console.log(`页面切换完成，当前页: ${currentPage.value}`)
      console.log(`图片缓存状态:`, pageImages.value.has(pageNum))
    })
  }
}

const previousPage = () => {
  if (currentPage.value > 1) {
    goToPage(currentPage.value - 1)
  }
}

const nextPage = () => {
  if (currentPage.value < totalPages.value) {
    goToPage(currentPage.value + 1)
  }
}

const togglePageSelection = (pageNum: number) => {
  const selected = [...props.selectedPages]
  const index = selected.indexOf(pageNum)
  
  if (index > -1) {
    selected.splice(index, 1)
  } else {
    selected.push(pageNum)
  }
  
  selected.sort((a, b) => a - b)
  emit('page-select', selected)
}

const isPageSelected = (pageNum: number) => {
  return props.selectedPages.includes(pageNum)
}

const toggleViewMode = () => {
  viewMode.value = viewMode.value === 'single' ? 'grid' : 'single'

  // 加载当前页图片
  if (hasDocument.value) {
    loadPageImage(currentPage.value)
  }
}

const getViewModeLabel = () => {
  return viewMode.value === 'single' ? '单页视图' : '网格视图'
}

const editPage = (pageNum: number) => {
  emit('edit-page', pageNum)
}

const processWithAI = (pageNum: number, forceReprocess = false) => {
  // 触发 AI 重新处理
  emit('process-pages', [pageNum], forceReprocess)
}

// 图片缩放控制
const zoomIn = () => {
  imageScale.value = Math.min(imageScale.value * 1.2, 3)
}

const zoomOut = () => {
  imageScale.value = Math.max(imageScale.value / 1.2, 0.3)
}

const resetZoom = () => {
  imageScale.value = 1
}

// 分割线拖拽控制
const startDrag = (event: MouseEvent) => {
  console.log('开始拖拽分割线')
  isDragging.value = true
  document.addEventListener('mousemove', onDrag, { passive: false })
  document.addEventListener('mouseup', stopDrag)
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'col-resize'
  event.preventDefault()
  event.stopPropagation()
}

const onDrag = (event: MouseEvent) => {
  if (!isDragging.value) return

  // 查找正确的容器 - 使用 .single-view 作为参考容器
  const container = document.querySelector('.single-view') as HTMLElement
  if (!container) {
    console.log('未找到 .single-view 容器')
    return
  }

  const rect = container.getBoundingClientRect()
  const mouseX = event.clientX
  const containerLeft = rect.left
  const containerWidth = rect.width

  // 计算相对于容器的位置百分比
  const relativeX = mouseX - containerLeft
  const newPosition = (relativeX / containerWidth) * 100

  console.log(`拖拽调试信息:`, {
    mouseX,
    containerLeft,
    containerWidth,
    relativeX,
    newPosition,
    currentSplitPosition: splitPosition.value
  })

  // 限制拖拽范围：左侧最小20%，最大80%
  const clampedPosition = Math.max(20, Math.min(80, newPosition))

  // 只有当位置真正改变时才更新
  if (Math.abs(clampedPosition - splitPosition.value) > 0.1) {
    splitPosition.value = clampedPosition
    console.log(`更新分割位置: ${clampedPosition}%`)
  }

  event.preventDefault()
}

const stopDrag = () => {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
}

// 预加载相邻页面
const preloadAdjacentPages = () => {
  if (currentPage.value > 1) {
    loadPageImage(currentPage.value - 1)
  }
  if (currentPage.value < totalPages.value) {
    loadPageImage(currentPage.value + 1)
  }
}

// 监听当前页变化，预加载相邻页面
watch(currentPage, () => {
  setTimeout(preloadAdjacentPages, 100)
})

// 打开图片模态对话框
const openImageModal = () => {
  showImageModal.value = true
}

// 关闭图片模态对话框
const closeImageModal = () => {
  showImageModal.value = false
}

// 切换到原生文本标签页并按需提取文本
const switchToOriginalTab = async () => {
  activeTab.value = 'original'

  // 如果当前页面没有原生文本且没有正在提取，则按需提取
  if (currentPageData.value && !currentPageData.value.text && !extractingNativeText.value) {
    await extractNativeTextForCurrentPage()
  }
}

// 提取当前页面的原生文本
const extractNativeTextForCurrentPage = async () => {
  if (!hasDocument.value || extractingNativeText.value) return

  try {
    extractingNativeText.value = true
    console.log(`开始提取第${currentPage.value}页原生文本`)

    // 调用后端API提取原生文本
    const text = await ExtractNativeText(currentPage.value)

    // 更新当前页面数据
    if (currentPageData.value) {
      currentPageData.value.text = text
      currentPageData.value.has_text = text && text.length > 0
    }

    console.log(`第${currentPage.value}页原生文本提取完成，长度: ${text ? text.length : 0}`)
  } catch (error) {
    console.error(`提取第${currentPage.value}页原生文本失败:`, error)
  } finally {
    extractingNativeText.value = false
  }
}

// 错误消息显示
const showErrorMessage = (message: string) => {
  console.error('AI处理错误:', message)
  // 这里可以使用更好的错误提示组件，暂时使用console.error
  // 可以添加toast提示或其他用户友好的错误显示方式
}

// AI处理相关方法
const closeAIPromptDialog = () => {
  showAIPromptDialog.value = false
  aiPrompt.value = ''
}

const editAIResult = (pageNumber: number) => {
  // 触发编辑AI结果事件
  emit('edit-page', pageNumber)
}

const startAIProcessing = async () => {
  if (!aiPrompt.value.trim() || processingAI.value) return

  // 检查当前页面是否有可处理的文本
  if (!currentPageData.value || (!currentPageData.value.ocr_text && !currentPageData.value.text)) {
    showErrorMessage('当前页面没有可处理的文本，请先进行OCR识别或提取原生文本')
    return
  }

  // 立即关闭对话框并开始处理
  const promptText = aiPrompt.value
  closeAIPromptDialog()

  // 切换到AI处理结果标签页
  activeTab.value = 'ai'

  try {
    processingAI.value = true
    aiProcessingMessage.value = '正在连接AI服务，请稍候...'

    // 只处理当前页面（单页模式）
    const pagesToProcess = [currentPage.value]

    console.log(`开始AI处理第${currentPage.value}页，提示词: ${promptText}`)

    // 创建一个Promise来等待AI处理完成事件
    const aiProcessingPromise = new Promise((resolve, reject) => {
      let completed = false
      const targetPage = currentPage.value

      const handleComplete = (data: any) => {
        console.log('AI处理完成事件:', data)
        if (!completed && data.pages && data.pages.includes(targetPage)) {
          completed = true
          // 通知父组件刷新文档数据以获取最新的AI处理结果
          emit('ai-processing-complete', {
            pages: data.pages,
            result: data.result
          })
          resolve(data)
        }
      }

      const handleError = (data: any) => {
        console.error('AI处理错误事件:', data)
        if (!completed) {
          completed = true
          reject(new Error(data.error || '未知错误'))
        }
      }

      // 使用一次性事件监听
      if (typeof window !== 'undefined' && (window as any).runtime?.EventsOn) {
        const runtime = (window as any).runtime

        // 创建一次性监听器
        const onceComplete = (data: any) => {
          handleComplete(data)
          // 移除监听器
          if (runtime.EventsOff) {
            runtime.EventsOff('ai-processing-complete', onceComplete)
            runtime.EventsOff('ai-processing-error', onceError)
          }
        }

        const onceError = (data: any) => {
          handleError(data)
          // 移除监听器
          if (runtime.EventsOff) {
            runtime.EventsOff('ai-processing-complete', onceComplete)
            runtime.EventsOff('ai-processing-error', onceError)
          }
        }

        runtime.EventsOn('ai-processing-complete', onceComplete)
        runtime.EventsOn('ai-processing-error', onceError)

        // 设置超时
        setTimeout(() => {
          if (!completed) {
            completed = true
            // 移除监听器
            if (runtime.EventsOff) {
              runtime.EventsOff('ai-processing-complete', onceComplete)
              runtime.EventsOff('ai-processing-error', onceError)
            }
            reject(new Error('AI处理超时'))
          }
        }, 60000) // 60秒超时
      } else {
        reject(new Error('运行时环境不支持事件监听'))
      }
    })

    // 调用后端API进行AI处理
    ProcessWithAI(pagesToProcess, promptText)

    // 等待AI处理完成事件
    await aiProcessingPromise

    // 处理完成
    aiProcessingMessage.value = '处理完成！'

    // 短暂显示完成状态
    await new Promise(resolve => setTimeout(resolve, 800))

  } catch (error) {
    console.error('AI处理失败:', error)

    // 解析错误信息
    let errorMessage = 'AI处理失败，请检查网络连接和AI服务配置'
    const errorStr = String(error)

    if (errorStr.includes('context deadline exceeded') || errorStr.includes('AI处理超时')) {
      errorMessage = 'AI处理超时，请检查网络连接或稍后重试'
    } else if (errorStr.includes('401') || errorStr.includes('Unauthorized')) {
      errorMessage = 'API密钥无效，请检查设置中的API Key配置'
    } else if (errorStr.includes('429') || errorStr.includes('rate limit')) {
      errorMessage = 'API请求频率过高，请稍后重试'
    } else if (errorStr.includes('500') || errorStr.includes('Internal Server Error')) {
      errorMessage = 'AI服务暂时不可用，请稍后重试'
    } else if (errorStr.includes('network') || errorStr.includes('fetch')) {
      errorMessage = '网络连接失败，请检查网络设置'
    }

    showErrorMessage(errorMessage)

  } finally {
    processingAI.value = false
    aiProcessingMessage.value = '正在连接AI服务...'
  }
}

// 监听AI处理事件
onMounted(() => {
  // 监听AI处理进度事件
  if (typeof window !== 'undefined' && (window as any).runtime?.EventsOn) {
    const runtime = (window as any).runtime

    runtime.EventsOn('ai-processing-progress', (data: any) => {
      console.log('AI处理进度:', data)
      // 可以在这里更新进度显示
    })

    // 监听AI处理完成事件（全局监听，用于其他地方触发的AI处理）
    runtime.EventsOn('ai-processing-complete', (data: any) => {
      console.log('AI处理完成（全局监听）:', data)
      // 这里不需要处理，因为startAIProcessing中已经有专门的处理逻辑
    })

    // 监听AI处理错误事件
    runtime.EventsOn('ai-processing-error', (data: any) => {
      console.error('AI处理错误:', data)
      processingAI.value = false
      alert(`AI处理失败: ${data.error || '未知错误'}`)
    })
  }
})
</script>

<template>
  <div class="pdf-viewer">
    <!-- 文件选择区域 -->
    <div v-if="!hasDocument" class="file-drop-zone">
      <div class="drop-content">
        <div class="drop-icon">📄</div>
        <h3>选择PDF文件</h3>
        <p>点击下方按钮选择要处理的PDF文件</p>
        <button @click="selectFile" class="btn btn-primary btn-large">
          选择文件
        </button>
      </div>
    </div>

    <!-- PDF查看器 -->
    <div v-else class="viewer-content">
      <!-- 工具栏 -->
      <div class="viewer-toolbar">
        <div class="toolbar-left">
          <button @click="selectFile" class="btn btn-secondary">
            更换文件
          </button>
          <span class="document-info">
            {{ document.file_path?.split('/').pop() || '未知文件' }}
          </span>
        </div>

        <div class="toolbar-center">
          <button
            v-if="viewMode === 'single'"
            @click="previousPage"
            :disabled="currentPage <= 1"
            class="btn btn-nav"
          >
            ←
          </button>
          <span v-if="viewMode === 'single'" class="page-info">
            <input
              v-model.number="currentPage"
              @change="goToPage(currentPage)"
              type="number"
              :min="1"
              :max="totalPages"
              class="page-input"
            />
            / {{ totalPages }}
          </span>
          <button
            v-if="viewMode === 'single'"
            @click="nextPage"
            :disabled="currentPage >= totalPages"
            class="btn btn-nav"
          >
            →
          </button>
          <div v-if="viewMode === 'grid'" class="grid-controls">
            <span class="grid-info">网格视图 - 共 {{ totalPages }} 页</span>
            <div class="grid-size-control">
              <label>图片大小:</label>
              <input
                type="range"
                v-model="gridSize"
                min="120"
                max="300"
                step="20"
                class="size-slider"
              />
              <span class="size-value">{{ gridSize }}px</span>
            </div>
          </div>
        </div>

        <div class="toolbar-right">
          <button @click="toggleViewMode" class="btn btn-secondary">
            切换到{{ viewMode === 'single' ? '网格视图' : '单页视图' }}
          </button>
          <span class="current-mode">当前: {{ getViewModeLabel() }}</span>
        </div>
      </div>

      <!-- 单页视图 -->
      <div v-if="viewMode === 'single'" class="single-view">
        <!-- 左侧预览区域 -->
        <div class="preview-panel" :style="{ width: splitPosition + '%' }">
          <div class="preview-header">
            <div class="page-selector">
              <input
                type="checkbox"
                :checked="isPageSelected(currentPage)"
                @change="togglePageSelection(currentPage)"
                class="page-checkbox"
              />
              <label>选择第{{ currentPage }}页进行处理</label>
            </div>

            <!-- 缩放控制 -->
            <div class="zoom-controls">
              <button @click="zoomOut" class="btn btn-small" title="缩小">-</button>
              <span class="zoom-level">{{ Math.round(imageScale * 100) }}%</span>
              <button @click="zoomIn" class="btn btn-small" title="放大">+</button>
              <button @click="resetZoom" class="btn btn-small" title="重置">重置</button>
            </div>
          </div>

          <!-- 图片预览区域 -->
          <div class="image-preview-container">
            <div class="image-wrapper" :style="{ transform: `scale(${imageScale})` }">
              <img
                v-if="pageImages.has(currentPage)"
                :src="pageImages.get(currentPage)"
                :alt="`第${currentPage}页`"
                class="preview-image"
                @dblclick="openImageModal"
                title="双击在大窗口中查看图片"
              />
              <div v-else-if="loading" class="loading-placeholder">
                <div class="spinner"></div>
                <p>加载中...</p>
              </div>
              <div v-else class="error-placeholder">
                <p>图片未加载</p>
                <button @click="loadPageImage(currentPage)" class="btn btn-small">
                  加载图片
                </button>
              </div>
            </div>
          </div>


        </div>

        <!-- 分割线 -->
        <div
          class="split-divider"
          :class="{ dragging: isDragging }"
          @mousedown="startDrag"
        ></div>

        <!-- 右侧结果区域 -->
        <div class="results-panel" :style="{ width: (100 - splitPosition) + '%' }">
          <!-- 页面信息 -->
          <div class="page-info-section">
            <h4>第 {{ currentPage }} 页信息</h4>
            <div v-if="currentPageData" class="page-meta">
              <span class="meta-item">
                <strong>尺寸:</strong>
                <span v-if="currentPageData.width > 0 && currentPageData.height > 0">
                  {{ Math.round(currentPageData.width) }} × {{ Math.round(currentPageData.height) }}
                </span>
                <span v-else class="size-pending">
                  渲染后获取
                </span>
              </span>
              <span class="meta-item">
                <strong>原生文本:</strong> {{ currentPageData.has_text ? '有' : '无' }}
              </span>
              <span class="meta-item">
                <strong>处理状态:</strong>
                <span :class="{ 'status-processed': currentPageData.processed, 'status-unprocessed': !currentPageData.processed }">
                  {{ currentPageData.processed ? '已处理' : '未处理' }}
                </span>
              </span>
            </div>
          </div>

          <!-- 解析结果展示 -->
          <div v-if="currentPageData" class="parsing-results">
            <div class="results-tabs">
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'ocr' }"
                @click="activeTab = 'ocr'"
              >
                OCR 识别结果
              </button>
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'ai' }"
                @click="activeTab = 'ai'"
              >
                AI 处理结果
              </button>
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'original' }"
                @click="switchToOriginalTab"
              >
                原生文本
              </button>
            </div>

            <div class="results-content">
              <div v-if="activeTab === 'ocr'" class="result-panel">
                <div class="result-header">
                  <h5>OCR 识别结果</h5>
                  <div class="header-buttons">
                    <button @click="processWithAI(currentPage, true)" class="btn btn-small btn-warning" title="重新进行OCR识别">
                      重新识别
                    </button>
                    <button @click="editPage(currentPage)" class="btn btn-small edit-btn">
                      编辑文本
                    </button>
                  </div>
                </div>
                <div class="result-text">
                  {{ currentPageData.ocr_text || '暂无 OCR 识别结果' }}
                </div>
              </div>

              <div v-if="activeTab === 'ai'" class="result-panel">
                <div class="result-header">
                  <h5>AI 处理结果</h5>
                  <div class="header-buttons">
                    <button
                      v-if="!processingAI && (currentPageData.ocr_text || currentPageData.text)"
                      @click="showAIPromptDialog = true"
                      class="btn btn-small btn-primary"
                      title="使用AI处理当前页面文本"
                    >
                      AI处理
                    </button>
                    <button
                      v-if="currentPageData.ai_text"
                      @click="editAIResult(currentPage)"
                      class="btn btn-small edit-btn"
                      title="编辑AI处理结果"
                    >
                      编辑结果
                    </button>
                  </div>
                </div>
                <div class="result-text">
                  <div v-if="processingAI" class="ai-processing-state">
                    <div class="processing-animation">
                      <div class="ai-spinner"></div>
                      <div class="processing-dots">
                        <span></span>
                        <span></span>
                        <span></span>
                      </div>
                    </div>
                    <div class="processing-info">
                      <h6>🤖 AI正在处理中</h6>
                      <p class="processing-message">{{ aiProcessingMessage }}</p>
                      <p class="processing-tip">请耐心等待，AI正在分析和处理您的文本内容...</p>
                    </div>
                  </div>
                  <div v-else-if="currentPageData.ai_text" class="markdown-content" v-html="renderedAIText">
                  </div>
                  <div v-else class="empty-state">
                    <div class="empty-icon">🤖</div>
                    <p class="empty-title">暂无AI处理结果</p>
                    <p class="empty-description">
                      您可以使用AI对OCR识别结果或原生文本进行处理，<br>
                      如纠错、总结、翻译、格式化等。
                    </p>
                    <button
                      v-if="currentPageData.ocr_text || currentPageData.text"
                      @click="showAIPromptDialog = true"
                      class="btn btn-primary"
                    >
                      开始AI处理
                    </button>
                    <p v-else class="empty-hint">
                      请先进行OCR识别或提取原生文本
                    </p>
                  </div>
                </div>
              </div>

              <div v-if="activeTab === 'original'" class="result-panel">
                <div class="result-header">
                  <h5>PDF 原生文本</h5>
                  <div class="header-buttons">
                    <button
                      v-if="!currentPageData.text && !extractingNativeText"
                      @click="extractNativeTextForCurrentPage"
                      class="btn btn-small btn-primary"
                      title="提取当前页面的原生文本"
                    >
                      提取原生文本
                    </button>
                  </div>
                </div>
                <div class="result-text">
                  <div v-if="extractingNativeText" class="loading-state">
                    <div class="spinner"></div>
                    <p>正在提取原生文本...</p>
                  </div>
                  <div v-else-if="currentPageData.text" class="text-content">
                    {{ currentPageData.text }}
                  </div>
                  <div v-else class="empty-state">
                    <div class="empty-icon">📄</div>
                    <p class="empty-title">该页面暂无原生文本</p>
                    <p class="empty-description">
                      PDF页面可能是扫描图片<br>
                      您可以点击按钮重试提取或使用OCR识别
                    </p>
                    <button
                      @click="extractNativeTextForCurrentPage"
                      class="btn btn-primary"
                    >
                      重试提取
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 网格视图 -->
      <div v-else-if="viewMode === 'grid'" class="grid-view">
        <div class="pages-grid" :style="{ '--grid-size': gridSize + 'px' }">
          <div
            v-for="pageNum in totalPages"
            :key="pageNum"
            class="grid-page"
            :class="{
              selected: isPageSelected(pageNum),
              current: pageNum === currentPage
            }"
            @click="goToPage(pageNum)"
          >
            <div class="grid-page-header">
              <input
                type="checkbox"
                :checked="isPageSelected(pageNum)"
                @change.stop="togglePageSelection(pageNum)"
                class="page-checkbox"
              />
              <span>第{{ pageNum }}页</span>
            </div>
            <div class="grid-page-image">
              <img
                v-if="pageImages.has(pageNum)"
                :src="pageImages.get(pageNum)"
                :alt="`第${pageNum}页`"
                @click.stop="loadPageImage(pageNum)"
              />
              <div v-else class="grid-placeholder" @click.stop="loadPageImage(pageNum)">
                <div class="placeholder-content">
                  <span class="placeholder-text">第{{ pageNum }}页</span>
                  <button class="btn btn-small">点击加载</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 图片查看模态对话框 -->
    <div v-if="showImageModal" class="image-modal-overlay" @click="closeImageModal">
      <div class="image-modal" @click.stop>
        <div class="modal-header">
          <h3>第{{ currentPage }}页 - 图片查看</h3>
          <div class="modal-controls">
            <!-- <span class="modal-tip">提示：右键图片可以复制或保存到本地</span> -->
            <!-- <button @click="closeImageModal" class="close-btn">&times;</button> -->
          </div>
        </div>
        <div class="modal-body">
          <div class="image-container">
            <img
              v-if="pageImages.has(currentPage)"
              :src="pageImages.get(currentPage)"
              :alt="`第${currentPage}页`"
              class="modal-image"
              draggable="true"
            />
          </div>
        </div>
        <div class="modal-footer">
          <div class="image-info">
            <span>第{{ currentPage }}页 / 共{{ totalPages }}页</span>
          </div>
          <div class="modal-actions">
            <button @click="goToPage(currentPage - 1)" :disabled="currentPage <= 1" class="btn btn-nav">
              上一页
            </button>
            <button @click="goToPage(currentPage + 1)" :disabled="currentPage >= totalPages" class="btn btn-nav">
              下一页
            </button>
            <button @click="closeImageModal" class="btn btn-secondary">
              关闭
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- AI提示词输入对话框 -->
    <div v-if="showAIPromptDialog" class="modal-overlay" @click="closeAIPromptDialog">
      <div class="ai-prompt-modal" @click.stop>
        <div class="modal-header">
          <h3>AI处理设置 - 第{{ currentPage }}页</h3>
          <button @click="closeAIPromptDialog" class="close-btn">&times;</button>
        </div>
        <div class="modal-body">
          <div class="prompt-section">
            <label for="ai-prompt">处理指令：</label>
            <textarea
              id="ai-prompt"
              v-model="aiPrompt"
              placeholder="请输入AI处理指令，例如：&#10;- 纠正OCR识别错误&#10;- 总结文本内容&#10;- 翻译为英文&#10;- 格式化为Markdown&#10;- 提取关键信息"
              class="prompt-textarea"
              rows="6"
            ></textarea>
            <div class="prompt-presets">
              <span class="presets-label">常用指令：</span>
              <button
                v-for="preset in promptPresets"
                :key="preset.name"
                @click="aiPrompt = preset.prompt"
                class="preset-btn"
                :title="preset.description"
              >
                {{ preset.name }}
              </button>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button @click="closeAIPromptDialog" class="btn btn-secondary">
            取消
          </button>
          <button
            @click="startAIProcessing"
            class="btn btn-primary"
            :disabled="!aiPrompt.trim() || processingAI"
          >
            {{ processingAI ? '处理中...' : '开始处理' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 引入外部样式文件 */
@import '../styles/PDFViewer.css';
</style>