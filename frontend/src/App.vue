<script lang="ts" setup>
import { ref, onMounted, computed, watch } from 'vue'
import PDFViewer from './components/PDFViewer.vue'
import ConfigPanel from './components/ConfigPanel.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import ProgressPanel from './components/ProgressPanel.vue'
import ErrorHandler from './components/ErrorHandler.vue'
import TextEditor from './components/TextEditor.vue'
import { LoadDocument, GetCurrentDocument, ProcessPages, ProcessPagesForce, CheckProcessedPages, GetConfig, GetSupportedFormats, ExportProcessingResults, SaveFileWithDialog, SaveBinaryFileWithDialog, GetAppVersion, CheckSystemDependencies, GetInstallInstructions, CancelProcessing, ProcessWithAIBatch, ProcessWithAIBatchForce, ProcessWithAIBatchContext, ProcessWithAIBatchForceContext, CheckAIProcessedPages } from '../wailsjs/go/main/App'
import { EventsOn, BrowserOpenURL } from '../wailsjs/runtime/runtime'
import { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType } from 'docx'
import { renderMarkdown } from './utils/markdown'

// 响应式数据
const currentDocument = ref<any>(null)
const selectedPages = ref<number[]>([])
const showConfig = ref(false)
const showHistory = ref(false)
const showExportDialog = ref(false)
const exportFormat = ref(localStorage.getItem('exportFormat') || 'txt')
const exportTextType = ref(localStorage.getItem('exportTextType') || 'auto') // auto, ocr, ai
const isExportingAIResults = ref(false)
const lastSuccessMessage = ref('')
const lastSuccessTime = ref(0)
const showTextEditor = ref(false)
const editingPageNumber = ref(0)
const editingTabType = ref<string>('ocr')
const processing = ref(false)
const appVersionInfo = ref<any>(null)
const systemDependencies = ref<any>(null)
const showDependencyWarning = ref(false)
const showAboutDialog = ref(false)

// 编辑器拖拽相关状态
const editorPosition = ref({ x: 0, y: 0 }) // 初始位置，将在显示时计算居中位置
const editorSize = ref({ width: 800, height: 600 })
const editorDragging = ref(false)
const editorDragOffset = ref({ x: 0, y: 0 })
const isResizingEditor = ref(false)
const editorResizeDirection = ref('')
const minEditorSize = { width: 400, height: 300 }
const supportedFormats = ref<string[]>([])
const progress = ref({
  total: 0,
  processed: 0,
  currentPage: 0,
  status: ''
})
const processingState = ref(0) // 0: idle, 1: running, 2: paused, 3: cancelling
const showProcessConfirmDialog = ref(false)
const processConfirmData = ref<any>(null)
const showAIConfirmDialog = ref(false)
const aiConfirmData = ref<any>(null)

// 从localStorage加载上次的导出格式
const loadLastExportFormat = () => {
  const saved = localStorage.getItem('app_exportFormat')
  if (saved && ['txt', 'markdown', 'html', 'rtf', 'docx'].includes(saved)) {
    exportFormat.value = saved
  }
}

// 保存导出格式到localStorage
const saveExportFormat = (format: string) => {
  localStorage.setItem('app_exportFormat', format)
}

// 检查是否首次启动
const checkFirstLaunch = () => {
  const hasLaunched = localStorage.getItem('app_hasLaunched')
  if (!hasLaunched) {
    // 首次启动，显示关于对话框
    showAboutDialog.value = true
    // 标记已启动过
    localStorage.setItem('app_hasLaunched', 'true')
  }
}

// 手动显示关于对话框
const showAbout = () => {
  showAboutDialog.value = true
}

// 关闭关于对话框
const closeAboutDialog = () => {
  showAboutDialog.value = false
}

// 打开使用帮助链接
const openHelp = () => {
  // 您可以在这里配置帮助链接地址
  const helpUrl = 'https://pdfseer.netlify.app' // 请替换为您的帮助文档链接
  BrowserOpenURL(helpUrl)
}

// 生命周期
onMounted(async () => {
  loadLastExportFormat()

  // 检查首次启动
  checkFirstLaunch()

  // 加载支持的格式
  try {
    supportedFormats.value = await GetSupportedFormats()
  } catch (error) {
    console.error('获取支持格式失败:', error)
  }

  // 获取应用版本信息
  try {
    appVersionInfo.value = await GetAppVersion()
  } catch (error) {
    console.error('获取版本信息失败:', error)
  }

  // 加载版本信息
  try {
    appVersionInfo.value = await GetAppVersion()
    console.log('应用版本信息:', appVersionInfo.value)
  } catch (error) {
    console.error('获取版本信息失败:', error)
  }

  // 监听事件
  EventsOn('dependency-check', (data: any) => {
    systemDependencies.value = data
    console.log('系统依赖检查结果:', data)

    // 检查是否有必需依赖缺失
    const missingRequired = data.dependencies?.some((dep: any) => dep.required && !dep.installed)
    if (missingRequired) {
      showDependencyWarning.value = true
    }
  })

  EventsOn('document-loaded', (data: any) => {
    // 重置页面选择状态（防止不同文档间的状态混乱）
    selectedPages.value = []

    currentDocument.value = data.document
    console.log('文档已加载:', data)
  })

  EventsOn('pdf-loaded', (data: any) => {
    currentDocument.value = data.document
    console.log('PDF已加载:', data)
  })

  EventsOn('processing-progress', (data: any) => {
    progress.value = data
    processing.value = true
    processingState.value = 1 // running
  })

  EventsOn('processing-complete', async (data: any) => {
    processing.value = false
    processingState.value = 0 // idle
    console.log('OCR处理完成:', data)

    // 使用统一的刷新机制，确保页面正确刷新，传递处理过的页面信息
    await refreshCurrentDocument(data.processedPages || [])

    // 显示完成提示
    if (data.total_processed) {
      window.dispatchEvent(new CustomEvent('show-success', {
        detail: `批量OCR处理完成：成功处理 ${data.total_processed} 页`
      }))
    } else {
      window.dispatchEvent(new CustomEvent('show-success', {
        detail: 'OCR处理完成'
      }))
    }
  })

  EventsOn('processing-error', (errorData: any) => {
    processing.value = false

    // 处理不同类型的错误
    if (typeof errorData === 'object' && errorData.code === 'DOCUMENT_NOT_LOADED') {
      // 文档未加载错误，提供重新加载选项
      window.dispatchEvent(new CustomEvent('show-error', {
        detail: {
          message: errorData.message || errorData.error,
          action: 'reload-document',
          title: '文档未加载'
        }
      }))
    } else {
      // 普通错误，由ErrorHandler组件处理
      const errorMessage = typeof errorData === 'string' ? errorData : (errorData.error || errorData.message || '处理失败')
      window.dispatchEvent(new CustomEvent('show-error', {
        detail: errorMessage
      }))
    }
  })

  EventsOn('ai-processing-complete', (data: any) => {
    console.log('AI处理完成:', data)

    // 关闭进度面板
    processing.value = false
    processingState.value = 0 // idle

    // 刷新当前文档数据
    refreshCurrentDocument()

    // 显示完成提示
    if (data.successCount && data.totalCount) {
      window.dispatchEvent(new CustomEvent('show-success', {
        detail: `批量AI处理完成：成功处理 ${data.successCount}/${data.totalCount} 页`
      }))
    } else {
      window.dispatchEvent(new CustomEvent('show-success', {
        detail: 'AI处理完成'
      }))
    }
  })

  EventsOn('processing-paused', (data: any) => {
    processingState.value = 2 // paused
    console.log('处理已暂停:', data)
    window.dispatchEvent(new CustomEvent('show-info', {
      detail: data.message || '批量处理已暂停'
    }))
  })

  // 监听单页OCR处理完成事件，实现实时刷新
  EventsOn('page-processed', async (data: any) => {
    console.log('单页OCR处理完成:', data)

    // 立即刷新文档数据以显示最新的处理结果
    try {
      const refreshedDoc = await GetCurrentDocument()
      if (refreshedDoc) {
        currentDocument.value = refreshedDoc
        console.log(`第${data.pageNumber}页OCR处理完成，文档数据已刷新`)

        // 通知 PDFViewer 保持当前页面，刷新指定页面
        window.dispatchEvent(new CustomEvent('document-refreshed', {
          detail: {
            document: refreshedDoc,
            keepCurrentPage: true,
            processedPages: [data.pageNumber]
          }
        }))
      }
    } catch (error) {
      console.error('刷新文档数据失败:', error)
    }
  })

  // 监听单页AI处理完成事件，实现实时刷新
  EventsOn('ai-page-processed', async (data: any) => {
    console.log('单页AI处理完成:', data)

    // 立即刷新文档数据以显示最新的AI处理结果
    try {
      const refreshedDoc = await GetCurrentDocument()
      if (refreshedDoc) {
        currentDocument.value = refreshedDoc
        console.log(`第${data.pageNumber}页AI处理完成，文档数据已刷新`)

        // 通知 PDFViewer 保持当前页面，刷新指定页面
        window.dispatchEvent(new CustomEvent('document-refreshed', {
          detail: {
            document: refreshedDoc,
            keepCurrentPage: true,
            processedPages: [data.pageNumber]
          }
        }))
      }
    } catch (error) {
      console.error('刷新文档数据失败:', error)
    }
  })

  EventsOn('processing-resumed', (data: any) => {
    processingState.value = 1 // running
    console.log('处理已继续:', data)
    window.dispatchEvent(new CustomEvent('show-success', {
      detail: data.message || '批量处理已继续'
    }))
  })

  EventsOn('processing-cancelled', (data: any) => {
    console.log('处理已取消:', data)
    // 前端已经在handleCancelProcessing中处理了flash提示和状态清理
    // 这里只记录日志即可
  })

  // 监听历史记录删除事件
  window.addEventListener('history-record-deleted', handleHistoryRecordDeleted)

  // 监听自定义消息事件
  window.addEventListener('show-error', (event: any) => {
    window.dispatchEvent(new CustomEvent('error', { detail: event.detail }))
  })

  window.addEventListener('show-warning', (event: any) => {
    window.dispatchEvent(new CustomEvent('warning', { detail: event.detail }))
  })

  window.addEventListener('show-info', (event: any) => {
    window.dispatchEvent(new CustomEvent('info', { detail: event.detail }))
  })

  window.addEventListener('show-success', (event: any) => {
    window.dispatchEvent(new CustomEvent('success', { detail: event.detail }))
  })

  // 监听导出AI结果事件
  window.addEventListener('export-ai-results', (event: any) => {
    const { pages } = event.detail
    handleExportAIResults(pages)
  })
})

// 监听导出格式变化，实时保存
watch(exportFormat, (newFormat) => {
  localStorage.setItem('exportFormat', newFormat)
})

// 监听导出格式变化，实时保存
watch(exportFormat, (newFormat) => {
  saveExportFormat(newFormat)
})

// 监听文本类型变化，实时保存
watch(exportTextType, (newType) => {
  localStorage.setItem('exportTextType', newType)
})

// 处理历史记录删除事件
const handleHistoryRecordDeleted = async (event: any) => {
  const { documentPath, documentName } = event.detail
  console.log('历史记录已删除:', documentName)

  // 如果删除的是当前加载的文档的历史记录，重新加载文档以确保状态同步
  if (currentDocument.value && currentDocument.value.filePath === documentPath) {
    console.log('当前文档的历史记录被删除，重新加载文档以确保状态同步')

    try {
      // 重新加载当前文档
      await LoadDocument(documentPath)

      // 获取重新加载后的文档
      const reloadedDoc = await GetCurrentDocument()
      if (reloadedDoc) {
        currentDocument.value = reloadedDoc
        console.log('文档重新加载成功')

        // 通知用户文档已重新加载
        window.dispatchEvent(new CustomEvent('show-info', {
          detail: '文档已重新加载，可以继续处理'
        }))
      }
    } catch (error) {
      console.error('重新加载文档失败:', error)
      window.dispatchEvent(new CustomEvent('show-error', {
        detail: '文档重新加载失败，请手动重新选择文件'
      }))
    }
  }
}

// 计算属性
const hasProcessedPages = computed(() => {
  return currentDocument.value?.pages?.some((page: any) => page.processed) || false
})

// 根据选择的文本类型计算可导出页面数
const getExportablePageCount = () => {
  if (!currentDocument.value?.pages) return 0

  if (isExportingAIResults.value) {
    // AI结果导出：统计有AI文本的页面
    return currentDocument.value.pages.filter((page: any) =>
      page.ai_text && page.ai_text.trim().length > 0
    ).length
  }

  // 普通导出：根据选择的文本类型统计
  return currentDocument.value.pages.filter((page: any) => {
    if (exportTextType.value === 'ocr') {
      // 只统计有OCR文本的页面
      return page.ocr_text && page.ocr_text.trim().length > 0
    } else if (exportTextType.value === 'ai') {
      // 只统计有AI文本的页面
      return page.ai_text && page.ai_text.trim().length > 0
    } else {
      // 智能选择：统计有任意文本的页面
      return (page.ocr_text && page.ocr_text.trim().length > 0) ||
             (page.ai_text && page.ai_text.trim().length > 0) ||
             (page.text && page.text.trim().length > 0)
    }
  }).length
}

// 方法
const handleFileSelect = async (filePath: string) => {
  try {
    // 重置页面选择状态
    selectedPages.value = []

    await LoadDocument(filePath)
  } catch (error) {
    console.error('加载文档失败:', error)
    // 错误会被ErrorHandler组件处理
  }
}

const handlePageSelect = (pageNumbers: number[]) => {
  selectedPages.value = pageNumbers
}

const handleProcessPages = async (pageNumbers?: number[], forceReprocess = false) => {
  const pagesToProcess = pageNumbers || selectedPages.value
  if (pagesToProcess.length === 0) {
    window.dispatchEvent(new CustomEvent('show-warning', {
      detail: '请先选择要处理的页面'
    }))
    return
  }

  // 如果是强制重新处理，直接执行
  if (forceReprocess) {
    ProcessPagesForce(pagesToProcess)
    return
  }

  try {
    // 检查哪些页面已经处理过
    const checkResult = await CheckProcessedPages(pagesToProcess)

    if (checkResult.processed_count > 0) {
      // 有已处理的页面，显示确认对话框
      processConfirmData.value = {
        pagesToProcess,
        checkResult
      }
      showProcessConfirmDialog.value = true
    } else {
      // 没有已处理的页面，直接处理
      ProcessPages(pagesToProcess)
    }
  } catch (error) {
    console.error('检查页面状态失败:', error)
    // 检查失败时直接处理
    ProcessPages(pagesToProcess)
  }
}

// 确认处理（使用缓存）
const confirmProcessWithCache = () => {
  if (processConfirmData.value) {
    ProcessPages(processConfirmData.value.pagesToProcess)
  }
  showProcessConfirmDialog.value = false
  processConfirmData.value = null
}

// 确认强制重新处理
const confirmProcessForce = () => {
  if (processConfirmData.value) {
    ProcessPagesForce(processConfirmData.value.pagesToProcess)
  }
  showProcessConfirmDialog.value = false
  processConfirmData.value = null
}

// 取消处理
const cancelProcess = () => {
  showProcessConfirmDialog.value = false
  processConfirmData.value = null
}

// AI确认处理（使用缓存）
const confirmAIProcessWithCache = async () => {
  if (aiConfirmData.value) {
    const unprocessedPages = aiConfirmData.value.unprocessedPages

    // 检查是否有未处理的页面
    if (unprocessedPages.length === 0) {
      // 所有页面都已处理，触发导出AI结果
      const processedPages = aiConfirmData.value.processedPages

      showAIConfirmDialog.value = false
      aiConfirmData.value = null

      // 通知PDFViewer关闭批量处理弹窗
      window.dispatchEvent(new CustomEvent('close-batch-ai-dialog'))

      // 触发导出AI处理结果
      handleExportAIResults(processedPages)
      return
    }

    // 有未处理的页面，开始处理
    await startBatchAIProcessing(unprocessedPages, aiConfirmData.value.prompt, false, aiConfirmData.value.contextMode)

    // 关闭AI确认弹窗和批量处理弹窗
    showAIConfirmDialog.value = false
    aiConfirmData.value = null

    // 通知PDFViewer关闭批量处理弹窗
    window.dispatchEvent(new CustomEvent('close-batch-ai-dialog'))
  }
}

// AI确认强制重新处理
const confirmAIProcessForce = async () => {
  if (aiConfirmData.value) {
    // 重新处理所有页面
    await startBatchAIProcessing(aiConfirmData.value.allPages, aiConfirmData.value.prompt, true, aiConfirmData.value.contextMode)

    // 关闭AI确认弹窗和批量处理弹窗
    showAIConfirmDialog.value = false
    aiConfirmData.value = null

    // 通知PDFViewer关闭批量处理弹窗
    window.dispatchEvent(new CustomEvent('close-batch-ai-dialog'))
  }
}

// 取消AI处理（保持批量处理弹窗打开）
const cancelAIProcess = () => {
  showAIConfirmDialog.value = false
  aiConfirmData.value = null
  // 不关闭批量处理弹窗，用户可以继续操作
}

// 暂停处理
const handlePauseProcessing = () => {
  processingState.value = 2 // paused
}

// 继续处理
const handleResumeProcessing = () => {
  processingState.value = 1 // running
}

// 取消处理
const handleCancelProcessing = async () => {
  try {
    processingState.value = 3 // cancelling

    // 调用后端取消方法
    await CancelProcessing()

    // 显示取消提示
    window.dispatchEvent(new CustomEvent('show-warning', {
      detail: '批量处理已取消'
    }))

    // 稍微延迟清理状态，让flash提示有时间显示
    setTimeout(() => {
      processing.value = false
      processingState.value = 0 // idle
      progress.value = {
        total: 0,
        processed: 0,
        currentPage: 0,
        status: ''
      }
    }, 100) // 100ms延迟，足够显示flash提示
  } catch (error) {
    console.error('取消处理失败:', error)
    // 如果后端调用失败，也要清理前端状态
    processing.value = false
    processingState.value = 0 // idle
    progress.value = {
      total: 0,
      processed: 0,
      currentPage: 0,
      status: ''
    }
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: '取消处理失败'
    }))
  }
}

// 格式化页面列表显示
const formatPageList = (pages: number[] | undefined) => {
  if (!pages || pages.length === 0) return '无'

  if (pages.length <= 5) {
    return pages.join(', ')
  }

  // 超过5页时显示前3页和后2页，中间用省略号
  const first = pages.slice(0, 3).join(', ')
  const last = pages.slice(-2).join(', ')
  return `${first} ... ${last}`
}

// 获取文档名（智能提取）
const getDocumentName = () => {
  if (!currentDocument.value) return '文档'

  // 优先使用title字段
  if (currentDocument.value.title && currentDocument.value.title.trim()) {
    return currentDocument.value.title.trim()
  }

  // 如果title为空，从file_path提取文件名
  if (currentDocument.value.file_path) {
    const fileName = currentDocument.value.file_path.split(/[/\\]/).pop() || '文档'
    // 移除文件扩展名
    return fileName.replace(/\.[^/.]+$/, '')
  }

  // 最后的备用方案
  return '文档'
}

const handleEditPage = (pageNumber: number, tabType?: string) => {
  editingPageNumber.value = pageNumber
  editingTabType.value = tabType || 'ocr' // 默认为OCR tab
  // 计算居中位置
  centerEditor()
  showTextEditor.value = true
}

const handleTextUpdated = (pageNumber: number, textType: string, text: string) => {
  // 更新当前文档的文本
  if (currentDocument.value && currentDocument.value.pages) {
    const page = currentDocument.value.pages.find((p: any) => p.number === pageNumber)
    if (page) {
      if (textType === 'ocr') {
        page.ocr_text = text
      } else if (textType === 'ai') {
        page.ai_text = text
      }
    }
  }
}

// 刷新当前文档数据
const refreshCurrentDocument = async (processedPages?: number[]) => {
  try {
    const refreshedDoc = await GetCurrentDocument()
    if (refreshedDoc) {
      currentDocument.value = refreshedDoc
      console.log('文档数据已刷新')

      // 通知 PDFViewer 保持当前页面，不要跳转
      window.dispatchEvent(new CustomEvent('document-refreshed', {
        detail: {
          document: refreshedDoc,
          keepCurrentPage: true,
          processedPages: processedPages || []
        }
      }))
    }
  } catch (error) {
    console.error('刷新文档数据失败:', error)
  }
}

const handlePageRendered = async (pageNumber: number) => {
  console.log(`页面 ${pageNumber} 已渲染，刷新文档数据以获取尺寸信息`)
  try {
    // 重新获取文档数据以更新页面尺寸信息
    const refreshedDoc = await GetCurrentDocument()
    if (refreshedDoc) {
      currentDocument.value = refreshedDoc
      console.log(`文档数据已刷新，页面 ${pageNumber} 尺寸信息已更新`)
    }
  } catch (error) {
    console.error('刷新文档数据失败:', error)
  }
}

const handleAIProcessingComplete = async (data: { pages: number[], result: string }) => {
  console.log('AI处理完成，刷新文档数据以获取最新的AI处理结果:', data)
  try {
    // 重新获取文档数据以更新AI处理结果
    const refreshedDoc = await GetCurrentDocument()
    if (refreshedDoc) {
      currentDocument.value = refreshedDoc
      console.log('文档数据已刷新，AI处理结果已更新')

      // 通知 PDFViewer 保持当前页面，不要跳转
      window.dispatchEvent(new CustomEvent('document-refreshed', {
        detail: {
          document: refreshedDoc,
          keepCurrentPage: true,
          processedPages: data.pages
        }
      }))
    }
  } catch (error) {
    console.error('刷新文档数据失败:', error)
  }
}

// 处理批量AI处理请求
const handleStartBatchAIProcessing = async (data: { pages: number[], prompt: string, contextMode?: boolean }) => {
  console.log('开始批量AI处理:', data)

  try {
    // 检查页面AI处理状态
    const checkResult = await CheckAIProcessedPages(data.pages)

    if (checkResult.processed_count > 0) {
      // 有已处理的页面，显示确认对话框
      const processedPages = checkResult.processed_pages as number[]
      const unprocessedPages = checkResult.unprocessed_pages as number[]

      // 显示AI处理确认对话框
      showAIConfirmDialog.value = true
      aiConfirmData.value = {
        totalPages: data.pages.length,
        processedPages: processedPages,
        unprocessedPages: unprocessedPages,
        allPages: data.pages,
        prompt: data.prompt,
        contextMode: data.contextMode || false
      }
    } else {
      // 没有已处理的页面，直接开始处理
      await startBatchAIProcessing(data.pages, data.prompt, false, data.contextMode)
    }

  } catch (error) {
    console.error('批量AI处理失败:', error)
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `批量AI处理失败: ${error}`
    }))
  }
}

// 实际开始批量AI处理
const startBatchAIProcessing = async (pages: number[], prompt: string, forceReprocess: boolean, contextMode?: boolean) => {
  try {
    // 显示进度面板
    processing.value = true
    processingState.value = 1 // processing
    progress.value = {
      total: pages.length,
      processed: 0,
      currentPage: 0,
      status: '准备开始AI处理...'
    }

    // 调用后端批量AI处理方法
    if (forceReprocess) {
      if (contextMode) {
        await ProcessWithAIBatchForceContext(pages, prompt, contextMode)
      } else {
        await ProcessWithAIBatchForce(pages, prompt)
      }
    } else {
      if (contextMode) {
        await ProcessWithAIBatchContext(pages, prompt, contextMode)
      } else {
        await ProcessWithAIBatch(pages, prompt)
      }
    }

  } catch (error) {
    console.error('批量AI处理失败:', error)
    processing.value = false
    processingState.value = 0 // idle

    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `批量AI处理失败: ${error}`
    }))
  }
}

const toggleConfig = () => {
  showConfig.value = !showConfig.value
}

const toggleHistory = () => {
  showHistory.value = !showHistory.value
}

// 计算编辑器居中位置
const centerEditor = () => {
  const windowWidth = window.innerWidth
  const windowHeight = window.innerHeight
  const editorWidth = editorSize.value.width
  const editorHeight = editorSize.value.height

  editorPosition.value = {
    x: Math.max(0, (windowWidth - editorWidth) / 2),
    y: Math.max(0, (windowHeight - editorHeight) / 2)
  }
}

const closeTextEditor = () => {
  showTextEditor.value = false
  editingPageNumber.value = 0
}

// 编辑器拖拽相关方法
const startDragEditor = (event: MouseEvent) => {
  editorDragging.value = true
  editorDragOffset.value = {
    x: event.clientX - editorPosition.value.x,
    y: event.clientY - editorPosition.value.y
  }

  document.addEventListener('mousemove', onDragEditor)
  document.addEventListener('mouseup', stopDragEditor)
  event.preventDefault()
}

const onDragEditor = (event: MouseEvent) => {
  if (!editorDragging.value) return

  editorPosition.value = {
    x: event.clientX - editorDragOffset.value.x,
    y: event.clientY - editorDragOffset.value.y
  }

  // 确保窗口不会拖拽到屏幕外，使用动态窗口尺寸
  const windowWidth = Math.min(window.innerWidth * 0.8, 1000)
  const windowHeight = Math.min(window.innerHeight * 0.7, 800)
  editorPosition.value.x = Math.max(0, Math.min(editorPosition.value.x, window.innerWidth - windowWidth))
  editorPosition.value.y = Math.max(0, Math.min(editorPosition.value.y, window.innerHeight - windowHeight))
}

const stopDragEditor = () => {
  editorDragging.value = false
  document.removeEventListener('mousemove', onDragEditor)
  document.removeEventListener('mouseup', stopDragEditor)
}

// 编辑器拉伸相关方法
const startResizeEditor = (event: MouseEvent, direction: string) => {
  event.preventDefault()
  event.stopPropagation()

  // 防止在拖拽时触发拉伸
  if (editorDragging.value) return

  isResizingEditor.value = true
  editorResizeDirection.value = direction

  document.addEventListener('mousemove', onResizeEditor)
  document.addEventListener('mouseup', stopResizeEditor)
}

const onResizeEditor = (event: MouseEvent) => {
  if (!isResizingEditor.value) return

  const direction = editorResizeDirection.value

  let newWidth = editorSize.value.width
  let newHeight = editorSize.value.height
  let newX = editorPosition.value.x
  let newY = editorPosition.value.y

  // 右边拉伸
  if (direction.includes('right')) {
    newWidth = Math.max(minEditorSize.width, event.clientX - editorPosition.value.x)
  }

  // 左边拉伸
  if (direction.includes('left')) {
    const newLeft = Math.min(event.clientX, editorPosition.value.x + editorSize.value.width - minEditorSize.width)
    newWidth = Math.max(minEditorSize.width, editorPosition.value.x + editorSize.value.width - newLeft)
    newX = newLeft
  }

  // 下边拉伸
  if (direction.includes('bottom')) {
    newHeight = Math.max(minEditorSize.height, event.clientY - editorPosition.value.y)
  }

  // 上边拉伸
  if (direction.includes('top')) {
    const newTop = Math.min(event.clientY, editorPosition.value.y + editorSize.value.height - minEditorSize.height)
    newHeight = Math.max(minEditorSize.height, editorPosition.value.y + editorSize.value.height - newTop)
    newY = newTop
  }

  // 确保不超出屏幕边界
  newX = Math.max(0, Math.min(newX, window.innerWidth - newWidth))
  newY = Math.max(0, Math.min(newY, window.innerHeight - newHeight))

  editorSize.value = { width: newWidth, height: newHeight }
  editorPosition.value = { x: newX, y: newY }
}

const stopResizeEditor = () => {
  isResizingEditor.value = false
  editorResizeDirection.value = ''
  document.removeEventListener('mousemove', onResizeEditor)
  document.removeEventListener('mouseup', stopResizeEditor)
}

const handleExport = async () => {
  try {
    // 检查是否有可导出的内容
    const exportableCount = getExportablePageCount()
    if (exportableCount === 0) {
      window.dispatchEvent(new CustomEvent('warning', {
        detail: '当前选择的文本类型没有可导出的内容，请选择其他文本类型或确保文档已处理'
      }))
      return
    }

    // 生成默认文件名，根据导出类型添加标识
    const timestamp = getLocalTimestamp()
    let typeLabel = ''

    if (isExportingAIResults.value) {
      typeLabel = '_AI批量处理'
    } else {
      // 根据选择的文本类型添加标识
      if (exportTextType.value === 'ocr') {
        typeLabel = '_OCR识别'
      } else if (exportTextType.value === 'ai') {
        typeLabel = '_AI处理'
      } else {
        typeLabel = '_智能选择'
      }
    }

    const defaultFileName = `${currentDocument.value?.title || 'PDF处理结果'}${typeLabel}_${timestamp}.${exportFormat.value}`

    if (exportFormat.value === 'docx') {
      // 显示生成提示
      window.dispatchEvent(new CustomEvent('show-info', {
        detail: '正在生成DOCX文档，请稍候...'
      }))

      // 生成DOCX内容
      const docxContent = await generateDocxContent()

      // 使用后端二进制保存对话框
      const filePath = await SaveBinaryFileWithDialog(docxContent, defaultFileName, [
        {
          DisplayName: 'Word文档',
          Pattern: '*.docx'
        }
      ])

      if (!filePath) {
        showExportDialog.value = false
        isExportingAIResults.value = false
        return
      }

      showExportDialog.value = false
      isExportingAIResults.value = false

      showSuccessMessage(`导出成功：${filePath}`)
    } else {
      // 其他格式使用前端生成内容
      const result = await generateExportContent(exportFormat.value)

      const filePath = await SaveFileWithDialog(result, defaultFileName, [
        {
          DisplayName: getFormatDisplayName(exportFormat.value),
          Pattern: `*.${exportFormat.value}`
        }
      ])

      if (!filePath) {
        isExportingAIResults.value = false
        return
      }

      showExportDialog.value = false
      isExportingAIResults.value = false

      showSuccessMessage(`导出成功：${filePath}`)
    }
  } catch (error) {
    console.error('导出失败:', error)
    isExportingAIResults.value = false
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `导出失败：${error}`
    }))
  }
}

// 导出AI处理结果
const handleExportAIResults = async (pageNumbers: number[]) => {
  try {
    // 设置一个标志，表示这是AI结果导出
    isExportingAIResults.value = true

    // 如果用户没有保存过格式偏好，AI导出默认使用markdown
    if (!localStorage.getItem('exportFormat')) {
      exportFormat.value = 'markdown'
    }

    // 显示导出对话框
    showExportDialog.value = true

  } catch (error) {
    console.error('导出AI结果失败:', error)
    isExportingAIResults.value = false
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `导出AI结果失败：${error}`
    }))
  }
}

// 生成导出内容
const generateExportContent = async (format: string): Promise<string> => {
  if (!currentDocument.value || !currentDocument.value.pages) {
    throw new Error('没有可导出的内容')
  }

  // 获取所有已处理的页面
  const processedPages = currentDocument.value.pages.filter((page: any) => page.processed)

  if (processedPages.length === 0) {
    throw new Error('没有已处理的页面可以导出')
  }

  // 合并所有页面的文本
  let allText = ''
  for (let i = 0; i < processedPages.length; i++) {
    const page = processedPages[i]
    // 根据导出类型选择文本
    let text = ''
    if (isExportingAIResults.value) {
      // AI导出：根据格式决定是否渲染
      if (exportFormat.value === 'markdown') {
        // markdown格式：导出原始markdown源码
        text = page.ai_text || ''
      } else if (exportFormat.value === 'html') {
        // html格式：导出渲染后的HTML
        text = renderMarkdown(page.ai_text || '')
      } else {
        // 其他格式（txt、rtf）：导出渲染后转换的纯文本
        text = convertMarkdownToPlainText(page.ai_text || '')
      }
    } else {
      // 普通导出：根据用户选择的文本类型
      if (exportTextType.value === 'ocr') {
        // 只导出OCR文本
        text = page.ocr_text || ''
      } else if (exportTextType.value === 'ai') {
        // 只导出AI文本
        if (exportFormat.value === 'markdown') {
          text = page.ai_text || ''
        } else if (exportFormat.value === 'html') {
          text = renderMarkdown(page.ai_text || '')
        } else {
          text = convertMarkdownToPlainText(page.ai_text || '')
        }
      } else {
        // 智能选择：优先OCR，其次AI，最后原生
        text = page.ocr_text || page.ai_text || page.text || ''
      }
    }

    if (text) {
      if (i > 0) {
        allText += '\n\n' // 页面间分隔
      }
      allText += text
    }
  }

  return allText
}


// 将markdown转换为纯文本（通过HTML渲染）
const convertMarkdownToPlainText = (markdown: string): string => {
  if (!markdown) return ''

  try {
    // 首先渲染markdown为HTML
    const html = renderMarkdown(markdown)

    // 创建临时DOM元素来提取纯文本
    const tempDiv = document.createElement('div')
    tempDiv.innerHTML = html

    // 获取纯文本内容
    const plainText = tempDiv.textContent || tempDiv.innerText || ''

    // 清理多余的空行和空格
    return plainText
      .replace(/\n{3,}/g, '\n\n') // 多个换行符合并为两个
      .replace(/[ \t]+/g, ' ') // 多个空格合并为一个
      .trim()
  } catch (error) {
    console.error('转换markdown为纯文本失败:', error)
    // 如果转换失败，返回原始markdown
    return markdown
  }
}

// 将HTML转换为DOCX内容（保持格式）
const convertHtmlToDocxContent = (markdown: string): (Paragraph | Table)[] => {
  if (!markdown) return []

  try {
    // 渲染markdown为HTML
    const html = renderMarkdown(markdown)

    // 创建临时DOM元素
    const tempDiv = document.createElement('div')
    tempDiv.innerHTML = html

    const content: (Paragraph | Table)[] = []

    // 遍历所有子元素
    const processElement = (element: Element): void => {
      const tagName = element.tagName.toLowerCase()

      switch (tagName) {
        case 'h1':
        case 'h2':
        case 'h3':
        case 'h4':
        case 'h5':
        case 'h6':
          const level = parseInt(tagName.charAt(1))
          const runs = parseHtmlElement(element)
          content.push(new Paragraph({
            children: runs.map(run => new TextRun({
              ...run,
              bold: true,
              size: Math.max(32 - level * 3, 20) // 更大的字体差异
            })),
            spacing: { before: 300, after: 150 }
          }))
          break

        case 'p':
          const pRuns = parseHtmlElement(element)
          if (pRuns.length > 0) {
            content.push(new Paragraph({
              children: pRuns,
              spacing: { after: 150 }
            }))
          }
          break

        case 'ul':
        case 'ol':
          element.querySelectorAll('li').forEach((li) => {
            const liRuns = parseHtmlElement(li)
            if (liRuns.length > 0) {
              content.push(new Paragraph({
                children: liRuns,
                bullet: tagName === 'ul' ? { level: 0 } : undefined,
                numbering: tagName === 'ol' ? { reference: 'default', level: 0 } : undefined,
                spacing: { after: 100 }
              }))
            }
          })
          break

        case 'table':
          content.push(convertHtmlTableToDocx(element as HTMLTableElement))
          break

        case 'hr':
          // 跳过分隔线，不在DOCX中显示
          break

        case 'blockquote':
          const quoteRuns = parseHtmlElement(element)
          if (quoteRuns.length > 0) {
            content.push(new Paragraph({
              children: quoteRuns,
              indent: { left: 720 }, // 缩进
              spacing: { after: 150 },
              border: {
                left: {
                  color: 'CCCCCC',
                  size: 6,
                  style: 'single'
                }
              }
            }))
          }
          break

        case 'pre':
          // 代码块
          const codeRuns = parseHtmlElement(element)
          if (codeRuns.length > 0) {
            content.push(new Paragraph({
              children: codeRuns.map(run => new TextRun({
                ...run,
                font: 'Courier New',
                size: 18
              })),
              spacing: { before: 150, after: 150 },
              indent: { left: 360 }
            }))
          }
          break

        case 'div':
        case 'span':
          // 对于div和span，直接处理内容
          const divRuns = parseHtmlElement(element)
          if (divRuns.length > 0) {
            content.push(new Paragraph({
              children: divRuns,
              spacing: { after: 100 }
            }))
          }
          break

        default:
          // 对于其他元素，递归处理子元素
          Array.from(element.children).forEach(child => {
            processElement(child)
          })
          break
      }
    }

    // 处理所有子元素
    Array.from(tempDiv.children).forEach(child => {
      processElement(child)
    })

    return content.length > 0 ? content : [new Paragraph({
      children: [new TextRun('内容为空')]
    })]

  } catch (error) {
    console.error('转换HTML为DOCX内容失败:', error)
    return [new Paragraph({
      children: [new TextRun('内容转换失败')]
    })]
  }
}

// 解析HTML元素为TextRun数组（改进版，支持嵌套格式）
const parseHtmlElement = (element: Element): TextRun[] => {
  const runs: TextRun[] = []

  const processNode = (node: Node, inheritedFormat: any = {}): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = node.textContent || ''
      if (text.trim()) {
        runs.push(new TextRun({
          text: text,
          ...inheritedFormat
        }))
      }
    } else if (node.nodeType === Node.ELEMENT_NODE) {
      const elem = node as Element
      const tagName = elem.tagName.toLowerCase()

      // 合并当前元素的格式与继承的格式
      const currentFormat = { ...inheritedFormat }

      switch (tagName) {
        case 'strong':
        case 'b':
          currentFormat.bold = true
          break
        case 'em':
        case 'i':
          currentFormat.italics = true
          break
        case 'code':
          currentFormat.font = 'Courier New'
          currentFormat.size = 20
          break
        case 'u':
          currentFormat.underline = {}
          break
        case 'del':
        case 's':
          currentFormat.strike = true
          break
      }

      // 递归处理子节点，传递合并后的格式
      Array.from(elem.childNodes).forEach(child => {
        processNode(child, currentFormat)
      })
    }
  }

  Array.from(element.childNodes).forEach(child => {
    processNode(child)
  })

  return runs.length > 0 ? runs : [new TextRun(element.textContent || '')]
}

// 转换HTML表格为DOCX表格（改进版）
const convertHtmlTableToDocx = (table: HTMLTableElement): Table => {
  const rows: TableRow[] = []

  try {
    const tableRows = table.querySelectorAll('tr')

    tableRows.forEach((tr) => {
      const cells: TableCell[] = []
      const tableCells = tr.querySelectorAll('td, th')

      tableCells.forEach(cell => {
        const runs = parseHtmlElement(cell)
        const isHeader = cell.tagName.toLowerCase() === 'th'

        // 为表头和普通单元格应用不同的样式
        const cellRuns = isHeader ?
          runs.map(run => new TextRun({
            ...run,
            bold: true,
            size: 22
          })) :
          runs

        cells.push(new TableCell({
          children: [new Paragraph({
            children: cellRuns.length > 0 ? cellRuns : [new TextRun(cell.textContent || '')],
            alignment: isHeader ? 'center' : 'left'
          })],
          width: {
            size: Math.floor(100 / tableCells.length),
            type: WidthType.PERCENTAGE,
          },
          margins: {
            top: 100,
            bottom: 100,
            left: 150,
            right: 150,
          },
          shading: isHeader ? {
            fill: 'F0F0F0'  // 表头背景色
          } : undefined
        }))
      })

      if (cells.length > 0) {
        rows.push(new TableRow({
          children: cells,
          height: {
            value: 400,
            rule: 'atLeast'
          }
        }))
      }
    })

    return new Table({
      rows,
      width: {
        size: 100,
        type: WidthType.PERCENTAGE,
      },
      borders: {
        top: { style: 'single', size: 1, color: '000000' },
        bottom: { style: 'single', size: 1, color: '000000' },
        left: { style: 'single', size: 1, color: '000000' },
        right: { style: 'single', size: 1, color: '000000' },
        insideHorizontal: { style: 'single', size: 1, color: '000000' },
        insideVertical: { style: 'single', size: 1, color: '000000' },
      },
      margins: {
        top: 200,
        bottom: 200,
      }
    })
  } catch (error) {
    console.error('转换HTML表格失败:', error)
    return new Table({
      rows: [new TableRow({
        children: [new TableCell({
          children: [new Paragraph({
            children: [new TextRun('表格转换失败')]
          })]
        })]
      })],
    })
  }
}

// 生成本地时间戳
const getLocalTimestamp = (): string => {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  const hours = String(now.getHours()).padStart(2, '0')
  const minutes = String(now.getMinutes()).padStart(2, '0')
  const seconds = String(now.getSeconds()).padStart(2, '0')

  return `${year}-${month}-${day}_${hours}-${minutes}-${seconds}`
}

// 防重复的成功提示
const showSuccessMessage = (message: string) => {
  const now = Date.now()
  // 如果是相同消息且在1秒内，则忽略
  if (lastSuccessMessage.value === message && now - lastSuccessTime.value < 1000) {
    return
  }

  lastSuccessMessage.value = message
  lastSuccessTime.value = now

  window.dispatchEvent(new CustomEvent('show-success', {
    detail: message
  }))
}

// 关闭导出对话框
const closeExportDialog = () => {
  showExportDialog.value = false
  isExportingAIResults.value = false
  // 保存用户选择的导出格式
  localStorage.setItem('exportFormat', exportFormat.value)
}

const getFormatDisplayName = (format: string) => {
  switch (format) {
    case 'txt': return '文本文件'
    case 'markdown': return 'Markdown文件'
    case 'html': return 'HTML文件'
    case 'rtf': return 'RTF文档'
    case 'docx': return 'Word文档'
    default: return '文件'
  }
}

// 生成DOCX内容
const generateDocxContent = async (): Promise<string> => {
  try {
    if (!currentDocument.value || !currentDocument.value.pages) {
      throw new Error('没有可导出的内容')
    }

    // 获取所有已处理的页面，按页码排序
    const processedPages = currentDocument.value.pages
      .filter((page: any) => page.processed)
      .sort((a: any, b: any) => a.number - b.number)

    if (processedPages.length === 0) {
      throw new Error('没有已处理的页面可以导出')
    }

    // 按页面生成内容，每页一个section
    const sections: any[] = []
    for (let i = 0; i < processedPages.length; i++) {
      const page = processedPages[i]
      // 根据导出类型选择文本
      let text = ''
      if (isExportingAIResults.value) {
        // AI导出：DOCX格式使用HTML转换，保持格式
        const aiContent = convertHtmlToDocxContent(page.ai_text || '')
        sections.push({
          properties: {
            page: {
              size: {
                orientation: 'portrait',
              },
              pageNumbers: {
                start: 1,
                formatType: 'decimal',
              },
            },
          },
          children: aiContent,
        })
        continue // 跳过后面的普通处理逻辑
      } else {
        // 普通导出：根据用户选择的文本类型
        if (exportTextType.value === 'ocr') {
          // 只导出OCR文本
          text = page.ocr_text || ''
        } else if (exportTextType.value === 'ai') {
          // 只导出AI文本，DOCX格式需要特殊处理
          const aiContent = convertHtmlToDocxContent(page.ai_text || '')
          sections.push({
            properties: {
              page: {
                size: {
                  orientation: 'portrait',
                },
                pageNumbers: {
                  start: 1,
                  formatType: 'decimal',
                },
              },
            },
            children: aiContent,
          })
          continue // 跳过后面的普通处理逻辑
        } else {
          // 智能选择：优先OCR，其次AI，最后原生
          text = page.ocr_text || page.ai_text || page.text || ''
        }
      }

      if (text) {
        // 检测当前页面是否包含表格
        const hasTable = detectTable(text)

        sections.push({
          properties: {
            page: {
              size: {
                orientation: 'portrait',
              },
              pageNumbers: {
                start: 1,
                formatType: 'decimal',
              },
            },
          },
          children: [
            ...(hasTable ? generateTableContent(text) : generateTextContent(text))
          ],
        })
      }
    }

    const doc = new Document({
      sections: sections,
    })

    // 生成文档
    const blob = await Packer.toBlob(doc)
    const arrayBuffer = await blob.arrayBuffer()
    const uint8Array = new Uint8Array(arrayBuffer)
    const binaryString = Array.from(uint8Array, byte => String.fromCharCode(byte)).join('')
    const base64String = btoa(binaryString)

    return base64String
  } catch (error) {
    console.error('DOCX生成失败:', error)
    throw error
  }
}

// 检测文本中是否包含表格
const detectTable = (text: string): boolean => {
  const lines = text.split('\n')

  // 检测明确的表格标记
  const explicitTablePatterns = [
    /\|.*\|.*\|/,       // 至少3个|分隔的表格（避免误判单个|）
    /┌.*┬.*┐/,          // 框线表格顶部
    /├.*┼.*┤/,          // 框线表格中间
    /\+[-=]{2,}\+[-=]{2,}\+/, // + 和 - 组成的表格边框（至少2个-）
  ]

  // 检查是否有明确的表格标记
  for (const line of lines) {
    if (explicitTablePatterns.some(pattern => pattern.test(line))) {
      return true
    }
  }

  // 检测Tab分隔的表格（需要多行且每行都有Tab）
  const tabLines = lines.filter(line => line.includes('\t') && line.split('\t').length >= 3)
  if (tabLines.length >= 2) {
    return true
  }

  // 检测多列对齐的表格（更严格的条件）
  const alignedLines = lines.filter(line => {
    // 检查是否有多个连续空格分隔的内容，且至少3列
    const parts = line.split(/\s{3,}/).filter(part => part.trim())
    return parts.length >= 3
  })

  // 只有当有多行（至少3行）且格式一致时才认为是表格
  if (alignedLines.length >= 3) {
    // 检查列数是否基本一致
    const columnCounts = alignedLines.map(line =>
      line.split(/\s{3,}/).filter(part => part.trim()).length
    )
    const avgColumns = columnCounts.reduce((a, b) => a + b, 0) / columnCounts.length
    const consistentColumns = columnCounts.every(count => Math.abs(count - avgColumns) <= 1)

    return consistentColumns
  }

  return false
}

// 生成表格内容
const generateTableContent = (text: string) => {
  const lines = text.split('\n').filter(line => line.trim())
  const content: (Paragraph | Table)[] = []

  let currentTable: string[] = []
  let inTable = false

  for (const line of lines) {
    if (line.trim() === '[PAGE_BREAK]') {
      // 处理分页符
      if (inTable && currentTable.length > 0) {
        // 结束当前表格
        content.push(createTableFromLines(currentTable))
        currentTable = []
        inTable = false
      }

      // 添加分页符
      content.push(new Paragraph({
        children: [new TextRun('')],
        pageBreakBefore: true
      }))
    } else if (detectTable(line)) {
      if (!inTable) {
        inTable = true
        currentTable = []
      }
      currentTable.push(line)
    } else {
      if (inTable && currentTable.length > 0) {
        // 结束当前表格，生成表格
        content.push(createTableFromLines(currentTable))
        currentTable = []
        inTable = false
      }

      // 添加普通段落
      if (line.trim()) {
        content.push(new Paragraph({
          children: [new TextRun(line)], // 使用默认格式
        }))
      }
    }
  }

  // 处理最后的表格
  if (inTable && currentTable.length > 0) {
    content.push(createTableFromLines(currentTable))
  }

  return content
}

// 从文本行创建表格
const createTableFromLines = (lines: string[]): Table => {
  const rows: TableRow[] = []

  try {
    for (const line of lines) {
      // 跳过分隔线
      if (/^[\s\-\+\=\|┌┐└┘├┤┬┴┼]*$/.test(line)) {
        continue
      }

      // 解析表格行
      let cells: string[] = []

      if (line.includes('|') && line.split('|').length >= 3) {
        // | 分隔的表格（至少3列）
        cells = line.split('|').map(cell => cell.trim()).filter(cell => cell)
      } else if (line.includes('\t') && line.split('\t').length >= 3) {
        // Tab分隔的表格（至少3列）
        cells = line.split('\t').map(cell => cell.trim()).filter(cell => cell)
      } else {
        // 空格分隔的表格（更严格的条件：至少3个空格分隔，且至少3列）
        const spaceSeparated = line.split(/\s{3,}/).map(cell => cell.trim()).filter(cell => cell)
        if (spaceSeparated.length >= 3) {
          cells = spaceSeparated
        }
      }

      // 只有当有足够的列时才创建表格行
      if (cells.length >= 2) {
        const tableCells = cells.map(cellText =>
          new TableCell({
            children: [new Paragraph({
              children: [new TextRun(cellText || ' ')] // 使用默认格式
            })],
            width: {
              size: Math.floor(100 / cells.length),
              type: WidthType.PERCENTAGE,
            },
          })
        )

        rows.push(new TableRow({
          children: tableCells
        }))
      }
    }

    // 如果没有有效行，创建一个简单的表格
    if (rows.length === 0) {
      rows.push(new TableRow({
        children: [new TableCell({
          children: [new Paragraph({
            children: [new TextRun('无法解析表格内容')]
          })]
        })]
      }))
    }

    return new Table({
      rows,
      width: {
        size: 100,
        type: WidthType.PERCENTAGE,
      },
    })
  } catch (error) {
    // 返回一个简单的表格作为后备
    return new Table({
      rows: [new TableRow({
        children: [new TableCell({
          children: [new Paragraph({
            children: [new TextRun('表格解析失败')]
          })]
        })]
      })],
      width: {
        size: 100,
        type: WidthType.PERCENTAGE,
      },
    })
  }
}





// 生成普通文本内容
const generateTextContent = (text: string) => {
  try {
    const lines = text.split('\n')
    const content: Paragraph[] = []

    for (const line of lines) {
      if (line.trim() === '[PAGE_BREAK]') {
        // 添加分页符
        content.push(new Paragraph({
          children: [new TextRun('')],
          pageBreakBefore: true
        }))
      } else {
        content.push(new Paragraph({
          children: [new TextRun(line || ' ')], // 使用默认字体和大小
        }))
      }
    }

    return content
  } catch (error) {
    // 返回一个简单的段落作为后备
    return [new Paragraph({
      children: [new TextRun('文本内容生成失败')]
    })]
  }
}
</script>

<template>
  <div class="app-container">
    <!-- 顶部工具栏 -->
    <header class="toolbar">
      <div class="toolbar-left">
        <h1>识文君</h1>
      </div>
      <div class="toolbar-right">
        <button @click="toggleHistory" class="btn btn-secondary">
          历史记录
        </button>
        <button @click="toggleConfig" class="btn btn-secondary">
          设置
        </button>
      </div>
    </header>

    <!-- 主内容区 -->
    <main class="main-content">
      <!-- 左侧边栏 -->
      <aside class="sidebar">
        <div class="sidebar-section">
          <h3>文档操作</h3>
          <div class="action-buttons">
            <button @click="handleProcessPages()"
                    :disabled="selectedPages.length === 0 || processing"
                    class="btn btn-primary">
              {{ processing ? '处理中...' : '开始处理' }}
            </button>

            <button @click="showExportDialog = true"
                    :disabled="!hasProcessedPages"
                    class="btn btn-secondary">
              导出结果
            </button>
          </div>
        </div>

        <div class="sidebar-section" v-if="currentDocument">
          <h3>页面选择</h3>
          <div class="page-selection">
            <p>已选择: {{ selectedPages.length }} 页</p>
            <div class="selection-buttons">
              <button @click="selectedPages = []" class="btn btn-small">
                清空选择
              </button>
              <button @click="selectedPages = Array.from({length: currentDocument.page_count}, (_, i) => i + 1)"
                      class="btn btn-small">
                全选
              </button>
            </div>
          </div>
        </div>

        <!-- 版权信息 -->
        <div class="sidebar-copyright">
          <div class="copyright-content">
            <div class="copyright-text">{{ appVersionInfo?.copyright || '© 2025 识文君 - PDF智能助手' }}</div>
              <div class="sidebar-links">
              <span class="about-link-sidebar" @click="showAbout">关于识文君</span>
              <span class="separator">|</span>
              <span class="help-link-sidebar" @click="openHelp">使用帮助</span>
            </div>
            <div class="author-info">{{ appVersionInfo ? `Developed by ${appVersionInfo.author}` : 'Developed by hzruo' }}</div>
            <div class="author-info">{{ appVersionInfo ? `Version: ${appVersionInfo.version}` : 'Version: 1.0.0' }}</div>
          </div>
        </div>
      </aside>

      <!-- PDF查看器 -->
      <div class="viewer-container">
        <PDFViewer
          :document="currentDocument"
          :selectedPages="selectedPages"
          :supportedFormats="supportedFormats"
          :processing="processing"
          @file-select="handleFileSelect"
          @page-select="handlePageSelect"
          @edit-page="handleEditPage"
          @process-pages="(pageNumbers, forceReprocess) => handleProcessPages(pageNumbers, forceReprocess)"
          @page-rendered="handlePageRendered"
          @ai-processing-complete="handleAIProcessingComplete"
          @start-batch-ai-processing="handleStartBatchAIProcessing"
        />
      </div>
    </main>

    <!-- 配置面板 -->
    <ConfigPanel v-if="showConfig" @close="showConfig = false" />

    <!-- 历史记录面板 -->
    <HistoryPanel v-if="showHistory" @close="showHistory = false" />

    <!-- 文本编辑器 - 可拖拽拉伸浮动窗口 -->
    <div v-if="showTextEditor" class="text-editor-overlay">
      <div
        class="text-editor-modal"
        :style="{
          left: editorPosition.x + 'px',
          top: editorPosition.y + 'px',
          width: editorSize.width + 'px',
          height: editorSize.height + 'px'
        }"
      >
        <!-- 拉伸手柄 -->
        <div class="resize-handle resize-top" @mousedown="startResizeEditor($event, 'top')"></div>
        <div class="resize-handle resize-right" @mousedown="startResizeEditor($event, 'right')"></div>
        <div class="resize-handle resize-bottom" @mousedown="startResizeEditor($event, 'bottom')"></div>
        <div class="resize-handle resize-left" @mousedown="startResizeEditor($event, 'left')"></div>
        <div class="resize-handle resize-top-left" @mousedown="startResizeEditor($event, 'top-left')"></div>
        <div class="resize-handle resize-top-right" @mousedown="startResizeEditor($event, 'top-right')"></div>
        <div class="resize-handle resize-bottom-left" @mousedown="startResizeEditor($event, 'bottom-left')"></div>
        <div class="resize-handle resize-bottom-right" @mousedown="startResizeEditor($event, 'bottom-right')"></div>

        <div class="modal-header" @mousedown="startDragEditor">
          <div class="drag-handle">⋮⋮ 拖拽移动</div>
        </div>
        <div class="modal-content">
          <TextEditor
            :pageNumber="editingPageNumber"
            :originalText="currentDocument?.pages?.find((p: any) => p.number === editingPageNumber)?.text"
            :ocrText="currentDocument?.pages?.find((p: any) => p.number === editingPageNumber)?.ocr_text"
            :aiText="currentDocument?.pages?.find((p: any) => p.number === editingPageNumber)?.ai_text"
            :documentName="getDocumentName()"
            :initialTab="editingTabType"
            @text-updated="handleTextUpdated"
            @close="closeTextEditor"
          />
        </div>
      </div>
    </div>

    <!-- 导出对话框 -->
    <div v-if="showExportDialog" class="export-dialog-overlay">
      <div class="export-dialog">
        <div class="dialog-header">
          <h3>{{ isExportingAIResults ? '导出AI处理结果' : '导出处理结果' }}</h3>
          <button @click="closeExportDialog" class="close-btn">&times;</button>
        </div>

        <div class="dialog-content">
          <!-- 文本类型选择 -->
          <div class="text-type-selection" v-if="!isExportingAIResults">
            <label>文本类型：</label>
            <div class="text-type-options">
              <label class="text-type-option">
                <input type="radio" v-model="exportTextType" value="auto" />
                <span class="option-label">🎯 智能选择</span>
              </label>
              <label class="text-type-option">
                <input type="radio" v-model="exportTextType" value="ocr" />
                <span class="option-label">🔍 OCR文本</span>
              </label>
              <label class="text-type-option">
                <input type="radio" v-model="exportTextType" value="ai" />
                <span class="option-label">🤖 AI文本</span>
              </label>
            </div>
            <div class="text-type-description">
              <p v-if="exportTextType === 'auto'" class="type-desc">
                <strong>智能选择：</strong>优先导出OCR识别文本，其次AI处理文本，最后原生文本
              </p>
              <p v-else-if="exportTextType === 'ocr'" class="type-desc">
                <strong>OCR文本：</strong>只导出OCR识别的文本内容，适合需要原始识别结果的场景
              </p>
              <p v-else-if="exportTextType === 'ai'" class="type-desc">
                <strong>AI文本：</strong>只导出AI处理的文本内容，包含格式化、纠错等优化结果
              </p>
            </div>
          </div>

          <!-- 导出格式选择 -->
          <div class="format-selection">
            <label>导出格式：</label>
            <div class="format-options">
              <label class="format-option">
                <input type="radio" v-model="exportFormat" value="txt" />
                <div class="option-content">
                  <div class="option-title">📄 文本文件 (.txt)</div>
                </div>
              </label>

              <label class="format-option">
                <input type="radio" v-model="exportFormat" value="markdown" />
                <div class="option-content">
                  <div class="option-title">📝 Markdown (.md)</div>
                </div>
              </label>

              <label class="format-option">
                <input type="radio" v-model="exportFormat" value="docx" />
                <div class="option-content">
                  <div class="option-title">📄 Word文档 (.docx)</div>
                </div>
              </label>

              <label class="format-option">
                <input type="radio" v-model="exportFormat" value="html" />
                <div class="option-content">
                  <div class="option-title">🌐 HTML (.html)</div>
                </div>
              </label>

              <label class="format-option">
                <input type="radio" v-model="exportFormat" value="rtf" />
                <div class="option-content">
                  <div class="option-title">📋 RTF文档 (.rtf)</div>
                </div>
              </label>
            </div>
          </div>

          <div class="export-info">
            <p v-if="hasProcessedPages">
              <strong>可导出页面数：</strong>
              {{ getExportablePageCount() }} 页
            </p>
            <p v-if="getExportablePageCount() === 0 && !isExportingAIResults" class="no-content-warning">
              <span class="warning-icon">⚠️</span>
              当前选择的文本类型没有可导出的内容
            </p>
          </div>
        </div>

        <div class="dialog-actions">
          <button @click="closeExportDialog" class="btn btn-secondary">
            取消
          </button>
          <button
            @click="handleExport"
            class="btn btn-primary"
            :disabled="getExportablePageCount() === 0"
            :title="getExportablePageCount() === 0 ? '没有可导出的内容' : ''"
          >
            导出
          </button>
        </div>
      </div>
    </div>

    <!-- 处理确认对话框 -->
    <div v-if="showProcessConfirmDialog" class="process-confirm-dialog-overlay">
      <div class="process-confirm-dialog">
        <div class="dialog-header">
          <h3>⚠️ 检测到已处理页面</h3>
        </div>

        <div class="dialog-content">
          <div v-if="processConfirmData && processConfirmData.checkResult" class="confirm-info">
            <!-- 全部已处理的情况 -->
            <div v-if="processConfirmData.checkResult.processed_count === processConfirmData.checkResult.total_pages" class="all-processed">
              <div class="status-icon">✅</div>
              <p class="main-message">
                您选择的 <strong>{{ processConfirmData.checkResult.total_pages }}</strong> 页全部已经处理过
              </p>
              <p class="sub-message">
                可以快速从缓存加载，或选择重新处理以获得最新结果
              </p>
            </div>

            <!-- 部分已处理的情况 -->
            <div v-else class="partial-processed">
              <div class="status-icon">⚠️</div>
              <p class="main-message">
                您选择的 <strong>{{ processConfirmData.checkResult.total_pages }}</strong> 页中，
                有 <strong class="processed-count">{{ processConfirmData.checkResult.processed_count }}</strong> 页已经处理过
              </p>

              <div class="page-summary">
                <div class="summary-item processed">
                  <span class="count">{{ processConfirmData.checkResult.processed_count }}</span>
                  <span class="label">已处理</span>
                  <span class="pages-preview">{{ formatPageList(processConfirmData.checkResult.processed_pages) }}</span>
                </div>
                <div class="summary-item unprocessed">
                  <span class="count">{{ processConfirmData.checkResult.unprocessed_pages?.length || 0 }}</span>
                  <span class="label">未处理</span>
                  <span class="pages-preview">{{ formatPageList(processConfirmData.checkResult.unprocessed_pages) }}</span>
                </div>
              </div>
            </div>

            <div class="options-explanation">
              <div class="option-item">
                <div class="option-icon">⚡</div>
                <div class="option-content">
                  <strong>使用缓存</strong>
                  <span>快速加载已处理页面，仅处理未完成的页面</span>
                </div>
              </div>
              <div class="option-item">
                <div class="option-icon">🔄</div>
                <div class="option-content">
                  <strong>重新处理</strong>
                  <span>重新识别所有页面，获得最新结果（耗时较长）</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="dialog-actions">
          <button @click="cancelProcess" class="btn btn-cancel">
            取消
          </button>
          <button @click="confirmProcessWithCache" class="btn btn-cache">
            ⚡ 使用缓存
          </button>
          <button @click="confirmProcessForce" class="btn btn-reprocess">
            🔄 重新处理
          </button>
        </div>
      </div>
    </div>

    <!-- AI处理确认弹窗 -->
    <div v-if="showAIConfirmDialog" class="ai-confirm-popup-overlay">
      <div class="ai-confirm-popup" @click.stop>
        <div class="dialog-header">
          <h3>🤖 检测到已处理页面</h3>
        </div>

        <div class="dialog-content">
          <div v-if="aiConfirmData" class="confirm-info">
            <!-- 全部已处理的情况 -->
            <div v-if="aiConfirmData.processedPages.length === aiConfirmData.totalPages" class="all-processed">
              <div class="status-icon">✨</div>
              <p class="main-message">
                您选择的 <strong>{{ aiConfirmData.totalPages }}</strong> 页全部已有AI处理结果
              </p>
              <p class="sub-message">
                您可以在历史记录中查看处理结果，或选择重新处理以获得最新结果
              </p>
            </div>

            <!-- 部分已处理的情况 -->
            <div v-else class="partial-processed">
              <div class="status-icon">⚠️</div>
              <p class="main-message">
                您选择的 <strong>{{ aiConfirmData.totalPages }}</strong> 页中，
                有 <strong class="processed-count">{{ aiConfirmData.processedPages.length }}</strong> 页已有AI处理结果
              </p>

              <div class="page-summary">
                <div class="summary-item processed">
                  <span class="count">{{ aiConfirmData.processedPages.length }}</span>
                  <span class="label">已处理</span>
                  <span class="pages-preview">{{ formatPageList(aiConfirmData.processedPages) }}</span>
                </div>
                <div class="summary-item unprocessed">
                  <span class="count">{{ aiConfirmData.unprocessedPages?.length || 0 }}</span>
                  <span class="label">未处理</span>
                  <span class="pages-preview">{{ formatPageList(aiConfirmData.unprocessedPages) }}</span>
                </div>
              </div>
            </div>

            <div class="options-explanation">
              <div class="option-item cache">
                <div class="option-icon">⚡</div>
                <div class="option-content">
                  <strong v-if="aiConfirmData.processedPages.length === aiConfirmData.totalPages">导出结果</strong>
                  <strong v-else>使用缓存</strong>
                  <span v-if="aiConfirmData.processedPages.length === aiConfirmData.totalPages">所有页面都已处理，可直接导出AI处理结果</span>
                  <span v-else>只处理未处理的页面，已处理页面使用缓存结果（推荐）</span>
                </div>
              </div>
              <div class="option-item reprocess">
                <div class="option-icon">🔄</div>
                <div class="option-content">
                  <strong>重新处理</strong>
                  <span>重新AI处理所有页面，获得最新结果（耗时较长）</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="dialog-actions">
          <button @click="cancelAIProcess" class="btn btn-cancel">
            取消
          </button>
          <button @click="confirmAIProcessWithCache" class="btn btn-cache">
            <span v-if="aiConfirmData && aiConfirmData.processedPages.length === aiConfirmData.totalPages">📤 导出结果</span>
            <span v-else>⚡ 使用缓存</span>
          </button>
          <button @click="confirmAIProcessForce" class="btn btn-reprocess">
            🔄 重新处理
          </button>
        </div>
      </div>
    </div>

    <!-- 关于对话框 -->
    <div v-if="showAboutDialog" class="about-dialog-overlay" @click="closeAboutDialog">
      <div class="about-dialog" @click.stop>
        <div class="dialog-header">
          <h3>🎯 关于识文君</h3>
          <button @click="closeAboutDialog" class="close-btn">&times;</button>
        </div>

        <div class="dialog-content">
          <div class="about-content">
            <!-- 应用图标和名称 -->
            <div class="app-info">
              <div class="app-icon">📄</div>
              <h2>识文君</h2>
              <p class="app-subtitle">PDF智能识别助手</p>
            </div>

            <!-- 版本信息 -->
            <div class="version-info">
              <div class="info-item">
                <span class="label">版本：</span>
                <span class="value">{{ appVersionInfo?.version || '1.0.0' }}</span>
              </div>
              <div class="info-item">
                <span class="label">开发者：</span>
                <span class="value">{{ appVersionInfo?.author || 'hzruo' }}</span>
              </div>
              <div class="info-item">
                <span class="label">联系邮箱：</span>
                <span class="value email-link">{{ appVersionInfo?.email || 'support@pdfseer.com' }}</span>
              </div>
            </div>

            <!-- 功能介绍 -->
            <div class="features">
              <h4>✨ 主要功能</h4>
              <div class="features-grid">
                <div class="feature-card">
                  <div class="feature-icon">🔍</div>
                  <div class="feature-content">
                    <h5>OCR文字识别</h5>
                    <p>高精度文字识别技术</p>
                  </div>
                </div>
                <div class="feature-card">
                  <div class="feature-icon">🤖</div>
                  <div class="feature-content">
                    <h5>AI智能处理</h5>
                    <p>智能文本分析与优化</p>
                  </div>
                </div>
                <div class="feature-card">
                  <div class="feature-icon">📝</div>
                  <div class="feature-content">
                    <h5>多格式导出</h5>
                    <p>支持多种文档格式</p>
                  </div>
                </div>
                <div class="feature-card">
                  <div class="feature-icon">📋</div>
                  <div class="feature-content">
                    <h5>批量处理</h5>
                    <p>高效批量文档处理</p>
                  </div>
                </div>
                <div class="feature-card">
                  <div class="feature-icon">💾</div>
                  <div class="feature-content">
                    <h5>历史记录</h5>
                    <p>完整的处理历史管理</p>
                  </div>
                </div>
                <div class="feature-card">
                  <div class="feature-icon">⚙️</div>
                  <div class="feature-content">
                    <h5>灵活配置</h5>
                    <p>丰富的个性化设置</p>
                  </div>
                </div>
              </div>
            </div>

            <!-- 版权信息 -->
            <div class="copyright">
              <p>{{ appVersionInfo?.copyright || '© 2025 识文君 - PDF智能助手' }}</p>
              <p class="description">让PDF文档处理更智能、更高效</p>
            </div>

            <!-- 协议说明 -->
            <div class="license-info">
              <h4>📋 使用协议</h4>
              <div class="license-content">
                <div class="license-item">
                  <span class="license-title">🔒 隐私保护</span>
                  <p>本软件承诺保护用户隐私，所有文档处理均在本地进行，不会上传或存储您的文件内容。</p>
                </div>
                <div class="license-item">
                  <span class="license-title">⚖️ 使用条款</span>
                  <p>本软件个人免费使用，不得进行二次售卖或商业分发，请遵守相关法律法规。</p>
                </div>
                <div class="license-item">
                  <span class="license-title">🛡️ 免责声明</span>
                  <p>软件按"现状"提供，开发者不对使用过程中可能出现的数据丢失或其他问题承担责任。</p>
                </div>
                <div class="license-item">
                  <span class="license-title">📧 技术支持</span>
                  <p>如遇问题或建议，请发送邮件至 <span class="email-highlight">{{ appVersionInfo?.email || 'support@pdfseer.com' }}</span></p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="dialog-actions">
          <button @click="closeAboutDialog" class="btn btn-primary">
            开始使用
          </button>
        </div>
      </div>
    </div>

    <!-- 进度面板 -->
    <ProgressPanel
      v-if="processing"
      :progress="progress"
      :processing-state="processingState"
      @pause="handlePauseProcessing"
      @resume="handleResumeProcessing"
      @cancel="handleCancelProcessing"
    />

    <!-- 错误处理器 -->
    <ErrorHandler />
  </div>
</template>

<style scoped>
.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;
}

.app-container::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><defs><pattern id="grain" width="100" height="100" patternUnits="userSpaceOnUse"><circle cx="25" cy="25" r="1" fill="rgba(255,255,255,0.05)"/><circle cx="75" cy="75" r="1" fill="rgba(255,255,255,0.05)"/></pattern></defs><rect width="100" height="100" fill="url(%23grain)"/></svg>');
  pointer-events: none;
  z-index: 1;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  position: relative;
  z-index: 10;
  min-height: 60px;
}

.toolbar-left h1 {
  margin: 0;
  font-size: 1.8rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-weight: 700;
  letter-spacing: -0.5px;
}

.toolbar-right {
  display: flex;
  gap: 1rem;
}

.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
  position: relative;
  z-index: 5;
  margin: 1rem;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.1);
  overflow: hidden;
  min-height: 0;
}

.sidebar {
  width: 320px;
  min-width: 280px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-right: 1px solid rgba(255, 255, 255, 0.2);
  padding: 1.5rem;
  overflow-y: auto;
  position: relative;
  display: flex;
  flex-direction: column;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.sidebar::-webkit-scrollbar {
  width: 8px;
}

.sidebar::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.sidebar::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.sidebar::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.sidebar-section {
  margin-bottom: 2rem;
  background: rgba(255, 255, 255, 0.6);
  border-radius: 12px;
  padding: 1.5rem;
  border: 1px solid rgba(255, 255, 255, 0.3);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.05);
}

.sidebar-section h3 {
  margin: 0 0 1rem 0;
  font-size: 1.2rem;
  color: #333;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.sidebar-section h3::before {
  content: '📄';
  font-size: 1rem;
}

.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.page-selection p {
  margin: 0 0 1rem 0;
  color: #666;
  font-size: 0.9rem;
  line-height: 1.5;
}

.selection-buttons {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.viewer-container {
  flex: 1;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  position: relative;
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

/* 按钮禁用状态 */
.btn:disabled {
  background: #ccc !important;
  color: #666 !important;
  border-color: #ccc !important;
  cursor: not-allowed !important;
  transform: none !important;
  box-shadow: none !important;
  opacity: 0.6;
}

.btn:disabled::before {
  display: none;
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

.btn-secondary:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(108, 117, 125, 0.4);
}

.btn-secondary:disabled {
  background: #ccc;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.btn-warning {
  background: linear-gradient(135deg, #ffc107 0%, #fd7e14 100%);
  color: #212529;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.btn-warning:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(255, 193, 7, 0.4);
}

.btn-warning:disabled {
  background: #ccc;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
  border-color: #ccc;
}

.btn-small {
  padding: 0.5rem 1rem;
  font-size: 0.8rem;
  background: rgba(255, 255, 255, 0.9);
  color: #333;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.btn-small:hover {
  background: rgba(255, 255, 255, 1);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.text-editor-overlay {
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
  z-index: 1500;
  animation: fadeIn 0.3s ease;
}

.text-editor-container {
  width: 90%;
  max-width: 1000px;
  height: 90%;
  max-height: 800px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15), 0 8px 20px rgba(0, 0, 0, 0.1);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: slideIn 0.3s ease;
}

/* 导出对话框样式 */
.export-dialog-overlay {
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
  z-index: 1600;
  animation: fadeIn 0.3s ease;
}

.export-dialog {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15), 0 8px 20px rgba(0, 0, 0, 0.1);
  width: 90%;
  max-width: 500px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: slideIn 0.3s ease;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 1.5rem 1rem 1.5rem;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.1) 0%, rgba(118, 75, 162, 0.1) 100%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.dialog-header h3 {
  margin: 0;
  color: #333;
  font-weight: 600;
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

.dialog-content {
  padding: 1.5rem;
  background: rgba(255, 255, 255, 0.5);
  max-height: 60vh;
  overflow-y: auto;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.dialog-content::-webkit-scrollbar {
  width: 8px;
}

.dialog-content::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.dialog-content::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.dialog-content::-webkit-scrollbar-thumb:hover {
  background: #999;
}

/* 文本类型选择样式 */
.text-type-selection {
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.3);
}

.text-type-selection > label {
  display: block;
  margin-bottom: 0.75rem;
  font-weight: 600;
  color: #333;
  font-size: 1rem;
}

.text-type-options {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}

.text-type-option {
  display: flex;
  align-items: center;
  padding: 0.6rem 0.8rem;
  background: rgba(255, 255, 255, 0.8);
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.3s ease;
  position: relative;
  flex: 1;
  min-width: 0;
  justify-content: center;
}

.text-type-option:hover {
  background: rgba(255, 255, 255, 0.95);
  border-color: rgba(102, 126, 234, 0.3);
  transform: translateY(-1px);
  box-shadow: 0 3px 12px rgba(0, 0, 0, 0.1);
}

.text-type-option input[type="radio"] {
  margin-right: 0.4rem;
  transform: scale(1.1);
}

.text-type-option .option-label {
  font-weight: 500;
  font-size: 0.85rem;
  white-space: nowrap;
}

.text-type-option input[type="radio"]:checked + .option-label {
  color: #667eea;
  font-weight: 600;
}

.text-type-option:has(input[type="radio"]:checked) {
  background: rgba(102, 126, 234, 0.1);
  border-color: #667eea;
  box-shadow: 0 3px 12px rgba(102, 126, 234, 0.2);
}

.text-type-description {
  background: rgba(255, 255, 255, 0.6);
  border-radius: 6px;
  padding: 0.75rem;
  border-left: 3px solid #667eea;
}

.text-type-description .type-desc {
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.4;
  color: #555;
}

.text-type-description .type-desc strong {
  color: #667eea;
  font-weight: 600;
}

/* 响应式调整 */
@media (max-width: 600px) {
  .text-type-options {
    flex-direction: column;
  }

  .text-type-option {
    justify-content: flex-start;
  }
}

.format-selection label {
  display: block;
  margin-bottom: 1rem;
  font-weight: 500;
  color: #333;
}

.format-options {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}

.format-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem;
  border: 2px solid #e0e0e0;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.format-option:hover {
  border-color: #007bff;
  background: #f8f9ff;
}

.format-option input[type="radio"] {
  margin: 0;
  flex-shrink: 0;
}

.format-option input[type="radio"]:checked + .option-content {
  color: #007bff;
}

.format-option:has(input[type="radio"]:checked) {
  border-color: #007bff;
  background: #f8f9ff;
}

.option-content {
  flex: 1;
  min-width: 0;
}

.option-title {
  font-weight: 500;
  font-size: 0.9rem;
  overflow: hidden;
  text-overflow: ellipsis;
}

.export-info {
  background: #f8f9fa;
  padding: 0.75rem;
  border-radius: 4px;
  border-left: 4px solid #007bff;
  margin-bottom: 1rem;
}

.export-info p {
  margin: 0 0 0.25rem 0;
  color: #666;
  font-size: 0.9rem;
}

.export-info .no-content-warning {
  color: #e74c3c;
  font-weight: 500;
  margin: 0.25rem 0 0 0;
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.85rem;
  line-height: 1.4;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.export-info .warning-icon {
  font-size: 1rem;
  flex-shrink: 0;
  margin-top: 0.1rem;
}

.export-info p:last-child {
  margin-bottom: 0;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  padding: 1rem 1.5rem;
  background: #f8f9fa;
  border-top: 1px solid #e0e0e0;
}

/* 处理确认对话框样式 */
.process-confirm-dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1700;
  backdrop-filter: blur(2px);
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.process-confirm-dialog {
  background: white;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15), 0 8px 20px rgba(0, 0, 0, 0.1);
  width: 90%;
  max-width: 480px;
  max-height: 80vh;
  overflow: hidden;
  transform: scale(1);
  animation: slideIn 0.3s ease;
  border: 1px solid rgba(255, 255, 255, 0.2);
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

.process-confirm-dialog .dialog-header {
  padding: 1.5rem 1.5rem 1rem 1.5rem;
  background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%);
  border-bottom: none;
  text-align: center;
}

.process-confirm-dialog .dialog-header h3 {
  margin: 0;
  color: #856404;
  font-size: 1.2rem;
  font-weight: 600;
}

.process-confirm-dialog .dialog-content {
  padding: 0 1.5rem 1.5rem 1.5rem;
  max-height: 50vh;
  overflow-y: auto;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.process-confirm-dialog .dialog-content::-webkit-scrollbar {
  width: 8px;
}

.process-confirm-dialog .dialog-content::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.process-confirm-dialog .dialog-content::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.process-confirm-dialog .dialog-content::-webkit-scrollbar-thumb:hover {
  background: #999;
}

/* 状态显示样式 */
.all-processed,
.partial-processed {
  text-align: center;
  margin-bottom: 1.5rem;
}

.status-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.confirm-info .main-message {
  font-size: 1.1rem;
  margin-bottom: 0.5rem;
  color: #333;
}

.sub-message {
  color: #666;
  margin-bottom: 0;
}

.processed-count {
  color: #dc3545;
}

/* 页面摘要样式 */
.page-summary {
  display: flex;
  gap: 1rem;
  justify-content: center;
  margin-top: 1rem;
}

.summary-item {
  background: #f8f9fa;
  border-radius: 8px;
  padding: 1rem;
  text-align: center;
  flex: 1;
  max-width: 200px;
}

.summary-item.processed {
  border-left: 4px solid #28a745;
}

.summary-item.unprocessed {
  border-left: 4px solid #ffc107;
}

.summary-item .count {
  display: block;
  font-size: 1.5rem;
  font-weight: bold;
  color: #333;
}

.summary-item .label {
  display: block;
  font-size: 0.9rem;
  color: #666;
  margin: 0.25rem 0;
}

/* 响应式设计 - 支持 720p 到 4K */

/* 4K 和超宽屏优化 (2560px+) */
@media (min-width: 2560px) {
  .toolbar {
    padding: 1.5rem 3rem;
  }

  .toolbar-left h1 {
    font-size: 2.2rem;
  }

  .main-content {
    margin: 1.5rem;
  }

  .sidebar {
    width: 400px;
    min-width: 380px;
    padding: 2rem;
  }

  .sidebar-section h3 {
    font-size: 1.3rem;
  }

  .btn {
    padding: 1rem 2rem;
    font-size: 1rem;
  }
}

/* 2K 屏幕优化 (1920px-2559px) */
@media (min-width: 1920px) and (max-width: 2559px) {
  .toolbar {
    padding: 1.25rem 2.5rem;
  }

  .toolbar-left h1 {
    font-size: 2rem;
  }

  .main-content {
    margin: 1.25rem;
  }

  .sidebar {
    width: 360px;
    min-width: 340px;
    padding: 1.75rem;
  }

  .sidebar-section h3 {
    font-size: 1.2rem;
  }

  .btn {
    padding: 0.875rem 1.75rem;
    font-size: 0.95rem;
  }
}

/* 1080p 屏幕优化 (1366px-1919px) */
@media (min-width: 1366px) and (max-width: 1919px) {
  .toolbar {
    padding: 1rem 2rem;
  }

  .toolbar-left h1 {
    font-size: 1.8rem;
  }

  .main-content {
    margin: 1rem;
  }

  .sidebar {
    width: 320px;
    min-width: 300px;
    padding: 1.5rem;
  }
}

/* 小于 1366px 的屏幕 */
@media (max-width: 1365px) {
  .toolbar {
    padding: 0.75rem 1.5rem;
  }

  .toolbar-left h1 {
    font-size: 1.6rem;
  }

  .main-content {
    margin: 0.75rem;
  }

  .sidebar {
    width: 280px;
    min-width: 260px;
    padding: 1.25rem;
  }
}

@media (max-width: 1280px) {
  .toolbar {
    padding: 0.75rem 1rem;
  }

  .toolbar-left h1 {
    font-size: 1.5rem;
  }

  .toolbar-right {
    gap: 0.75rem;
  }

  .main-content {
    margin: 0.5rem;
  }

  .sidebar {
    width: 260px;
    min-width: 240px;
    padding: 1rem;
  }

  .sidebar-section {
    margin-bottom: 1.5rem;
  }

  .sidebar-section h3 {
    font-size: 1rem;
    margin-bottom: 0.75rem;
  }

  .btn {
    padding: 0.6rem 1.25rem;
    font-size: 0.85rem;
  }
}

@media (max-height: 768px) {
  .toolbar {
    padding: 0.5rem 1rem;
    min-height: 50px;
  }

  .toolbar-left h1 {
    font-size: 1.4rem;
  }

  .main-content {
    margin: 0.25rem;
  }

  .sidebar {
    padding: 0.75rem;
  }

  .sidebar-section {
    margin-bottom: 1rem;
  }

  .sidebar-section h3 {
    font-size: 0.95rem;
    margin-bottom: 0.5rem;
  }

  .action-buttons {
    gap: 0.5rem;
  }

  .btn {
    padding: 0.5rem 1rem;
    font-size: 0.8rem;
  }
}

@media (max-height: 720px) {
  .app-container {
    overflow-y: auto;
  }

  .main-content {
    min-height: calc(100vh - 60px);
    flex: none;
  }

  .sidebar {
    max-height: calc(100vh - 80px);
  }
}

.summary-item .pages-preview {
  display: block;
  font-size: 0.8rem;
  color: #007bff;
  font-family: monospace;
  word-break: break-all;
}

/* 选项说明样式 */
.options-explanation {
  border: 1px solid #e9ecef;
  border-radius: 8px;
  padding: 1rem;
  background: #fafafa;
}

.option-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  margin-bottom: 1rem;
  padding: 0.75rem;
  background: white;
  border-radius: 6px;
  border: 1px solid #e9ecef;
}

.option-item:last-child {
  margin-bottom: 0;
}

.option-icon {
  font-size: 1.2rem;
  flex-shrink: 0;
}

.option-content {
  flex: 1;
}

.option-content strong {
  display: block;
  color: #333;
  margin-bottom: 0.25rem;
}

.option-content span {
  color: #666;
  font-size: 0.9rem;
  line-height: 1.4;
}

/* 弹窗按钮样式 */
.process-confirm-dialog .dialog-actions {
  padding: 1rem 1.5rem 1.5rem 1.5rem;
  background: #fafafa;
  border-top: 1px solid #f0f0f0;
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}

.process-confirm-dialog .btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 8px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.btn-cancel {
  background: #f8f9fa;
  color: #6c757d;
  border: 1px solid #dee2e6;
}

.btn-cancel:hover {
  background: #e9ecef;
  color: #495057;
}

.btn-cache {
  background: linear-gradient(135deg, #28a745 0%, #20c997 100%);
  color: white;
  box-shadow: 0 2px 8px rgba(40, 167, 69, 0.3);
}

.btn-cache:hover {
  background: linear-gradient(135deg, #218838 0%, #1ea085 100%);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(40, 167, 69, 0.4);
}

.btn-reprocess {
  background: linear-gradient(135deg, #ffc107 0%, #fd7e14 100%);
  color: #212529;
  box-shadow: 0 2px 8px rgba(255, 193, 7, 0.3);
}

.btn-reprocess:hover {
  background: linear-gradient(135deg, #e0a800 0%, #e8630a 100%);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(255, 193, 7, 0.4);
}

/* 可拖拽编辑器窗口样式 */
.text-editor-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
  z-index: 1000;
  pointer-events: none; /* 允许点击穿透到背景 */
}

.text-editor-modal {
  position: absolute;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  pointer-events: auto; /* 恢复窗口内的点击事件 */
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
  min-width: 400px;
  min-height: 300px;
  max-width: 90vw;
  max-height: 90vh;
}

/* 拉伸手柄样式 */
.resize-handle {
  position: absolute;
  background: transparent;
  z-index: 10;
}

.resize-top {
  top: -3px;
  left: 10px;
  right: 10px;
  height: 6px;
  cursor: n-resize;
}

.resize-right {
  top: 10px;
  right: -3px;
  bottom: 10px;
  width: 6px;
  cursor: e-resize;
}

.resize-bottom {
  bottom: -3px;
  left: 10px;
  right: 10px;
  height: 6px;
  cursor: s-resize;
}

.resize-left {
  top: 10px;
  left: -3px;
  bottom: 10px;
  width: 6px;
  cursor: w-resize;
}

.resize-top-left {
  top: -3px;
  left: -3px;
  width: 10px;
  height: 10px;
  cursor: nw-resize;
}

.resize-top-right {
  top: -3px;
  right: -3px;
  width: 10px;
  height: 10px;
  cursor: ne-resize;
}

.resize-bottom-left {
  bottom: -3px;
  left: -3px;
  width: 10px;
  height: 10px;
  cursor: sw-resize;
}

.resize-bottom-right {
  bottom: -3px;
  right: -3px;
  width: 10px;
  height: 10px;
  cursor: se-resize;
}

.modal-header {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0.5rem 1rem;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
  cursor: move; /* 显示拖拽光标 */
  user-select: none; /* 防止文本选择 */
  min-height: 40px;
}

.drag-handle {
  color: #666;
  font-size: 0.8rem;
  font-weight: 500;
  letter-spacing: 1px;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.drag-handle:hover {
  color: #333;
}

.modal-content {
  flex: 1;
  overflow: hidden;
}

/* 侧边栏版权信息样式 */
.sidebar-copyright {
  margin-top: auto;
  padding-top: 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.3);
}

.sidebar-copyright .copyright-content {
  text-align: center;
  padding: 0.75rem;
  background: rgba(255, 255, 255, 0.4);
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  transition: all 0.3s ease;
}

.sidebar-copyright .copyright-content:hover {
  background: rgba(255, 255, 255, 0.6);
  border-color: rgba(102, 126, 234, 0.3);
  transform: translateY(-1px);
}

.sidebar-copyright .copyright-text {
  font-size: 0.75rem;
  color: #555;
  font-weight: 600;
  letter-spacing: 0.3px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  line-height: 1.3;
  margin-bottom: 2px;
}

.sidebar-copyright .author-info {
  font-size: 0.7rem;
  color: #777;
  font-weight: 400;
  letter-spacing: 0.2px;
  opacity: 0.8;
  line-height: 1.2;
}

.sidebar-copyright .copyright-content:hover .author-info {
  opacity: 1;
  color: #666;
}

/* 使用帮助对话框样式 */
.help-dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
  animation: fadeIn 0.3s ease;
}

.help-dialog {
  background: rgba(255, 255, 255, 0.98);
  backdrop-filter: blur(8px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  width: 90%;
  max-width: 600px;
  max-height: 85vh;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: popIn 0.4s cubic-bezier(0.68, -0.55, 0.265, 1.55);
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}

.help-dialog .dialog-header {
  background: rgba(248, 249, 250, 0.95);
  padding: 1.5rem;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-radius: 16px 16px 0 0;
}

.help-dialog .dialog-header h3 {
  margin: 0;
  color: #333;
  font-size: 1.3rem;
  font-weight: 600;
}

.help-dialog .close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #666;
  padding: 0.25rem;
  border-radius: 6px;
  transition: all 0.2s ease;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.help-dialog .close-btn:hover {
  background: rgba(0, 0, 0, 0.1);
  color: #333;
  transform: scale(1.1);
}

.help-dialog .dialog-content {
  padding: 0;
  max-height: 65vh;
  overflow-y: auto;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.help-dialog .dialog-content::-webkit-scrollbar {
  width: 6px;
}

.help-dialog .dialog-content::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 3px;
}

.help-dialog .dialog-content::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 3px;
}

.help-dialog .dialog-content::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.help-content {
  padding: 2rem;
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}

.help-section {
  margin-bottom: 2rem;
}

.help-section h4 {
  margin: 0 0 1rem 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
}

.help-section p {
  margin: 0 0 1rem 0;
  color: #555;
  font-size: 0.95rem;
  line-height: 1.6;
}

.help-dialog .dialog-actions {
  padding: 1.5rem;
  background: rgba(248, 249, 250, 0.95);
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: center;
  border-radius: 0 0 16px 16px;
}

.help-dialog .dialog-actions .btn {
  min-width: 120px;
  padding: 0.75rem 2rem;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 10px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
  color: white;
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 4px 15px rgba(102, 126, 234, 0.3);
}

.help-dialog .dialog-actions .btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(102, 126, 234, 0.4);
}

/* 关于对话框样式 */
.about-dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
  animation: fadeIn 0.3s ease;
}

.about-dialog {
  background: rgba(255, 255, 255, 0.98);
  backdrop-filter: blur(8px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  width: 90%;
  max-width: 520px;
  max-height: 85vh;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: popIn 0.4s cubic-bezier(0.68, -0.55, 0.265, 1.55);
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}

@keyframes popIn {
  0% {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
  }
  100% {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.about-dialog .dialog-header {
  background: rgba(248, 249, 250, 0.95);
  padding: 1.5rem;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-radius: 16px 16px 0 0;
}

.about-dialog .dialog-header h3 {
  margin: 0;
  color: #333;
  font-size: 1.3rem;
  font-weight: 600;
}

.about-dialog .close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #666;
  padding: 0.25rem;
  border-radius: 6px;
  transition: all 0.2s ease;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.about-dialog .close-btn:hover {
  background: rgba(0, 0, 0, 0.1);
  color: #333;
  transform: scale(1.1);
}

.about-dialog .dialog-content {
  padding: 0;
  max-height: 65vh;
  overflow-y: auto;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.about-dialog .dialog-content::-webkit-scrollbar {
  width: 6px;
}

.about-dialog .dialog-content::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 3px;
}

.about-dialog .dialog-content::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 3px;
}

.about-dialog .dialog-content::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.about-content {
  padding: 2rem;
  text-align: center;
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}

.app-info {
  margin-bottom: 2rem;
}

.app-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.1));
}

.app-info h2 {
  margin: 0 0 0.5rem 0;
  font-size: 2.2rem;
  color: #333;
  font-weight: 700;
  letter-spacing: -0.5px;
  text-shadow: 0 2px 4px rgba(102, 126, 234, 0.2);
}

.app-subtitle {
  margin: 0;
  color: #666;
  font-size: 1.1rem;
  font-weight: 500;
}

.version-info {
  background: #ffffff;
  border-radius: 12px;
  padding: 1.5rem;
  margin-bottom: 2rem;
  border: 1px solid #e0e0e0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
  font-size: 0.95rem;
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.info-item:last-child {
  margin-bottom: 0;
}

.info-item .label {
  color: #555;
  font-weight: 600;
  font-size: 0.9rem;
}

.info-item .value {
  color: #222;
  font-weight: 600;
  font-size: 0.9rem;
  /* 使用系统字体而不是等宽字体 */
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

.email-link {
  color: #667eea !important;
  cursor: pointer;
  transition: all 0.2s ease;
  /* 改善邮箱字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif !important;
  font-weight: 600 !important;
}

.email-link:hover {
  color: #764ba2 !important;
  text-decoration: underline;
}

.features {
  text-align: left;
  margin-bottom: 2rem;
}

.features h4 {
  margin: 0 0 1.5rem 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
  text-align: center;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
  margin-top: 1rem;
}

.feature-card {
  background: #ffffff;
  border-radius: 10px;
  padding: 1rem;
  border: 1px solid #e0e0e0;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.feature-card:hover {
  border-color: #667eea;
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.15);
  transform: translateY(-1px);
}

.feature-icon {
  font-size: 1.5rem;
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(102, 126, 234, 0.1);
  border-radius: 8px;
}

.feature-content {
  flex: 1;
  min-width: 0;
}

.feature-content h5 {
  margin: 0 0 0.25rem 0;
  color: #333;
  font-size: 0.9rem;
  font-weight: 600;
  line-height: 1.2;
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.feature-content p {
  margin: 0;
  color: #666;
  font-size: 0.8rem;
  line-height: 1.3;
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.copyright {
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  padding-top: 1.5rem;
  color: #666;
}

.copyright p {
  margin: 0 0 0.5rem 0;
  font-size: 0.9rem;
}

.copyright .description {
  font-style: italic;
  color: #999;
  font-size: 0.85rem;
}

/* 协议说明样式 */
.license-info {
  text-align: left;
  margin-top: 2rem;
  padding-top: 1.5rem;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
}

.license-info h4 {
  margin: 0 0 1.5rem 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
  text-align: center;
}

.license-content {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
}

.license-item {
  background: rgba(248, 249, 250, 0.9);
  border-radius: 10px;
  padding: 1rem;
  border: 1px solid rgba(0, 0, 0, 0.08);
  transition: all 0.2s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.license-item:hover {
  background: rgba(255, 255, 255, 0.95);
  border-color: rgba(102, 126, 234, 0.2);
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.license-title {
  display: block;
  font-weight: 600;
  color: #333;
  font-size: 0.95rem;
  margin-bottom: 0.5rem;
  line-height: 1.3;
}

.license-item p {
  margin: 0;
  color: #555;
  font-size: 0.85rem;
  line-height: 1.5;
}

.email-highlight {
  color: #667eea;
  font-weight: 600;
  background: rgba(102, 126, 234, 0.1);
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  /* 改善字体渲染 */
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  border: 1px solid rgba(102, 126, 234, 0.2);
}

.about-dialog .dialog-actions {
  padding: 1.5rem;
  background: rgba(248, 249, 250, 0.95);
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: center;
  border-radius: 0 0 16px 16px;
}

.about-dialog .dialog-actions .btn {
  min-width: 140px;
  padding: 0.75rem 2rem;
  font-size: 1rem;
  font-weight: 600;
  border-radius: 10px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
  color: white;
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 4px 15px rgba(102, 126, 234, 0.3);
}

.about-dialog .dialog-actions .btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(102, 126, 234, 0.4);
}

/* 响应式设计 */
@media (max-width: 480px) {
  .about-dialog {
    width: 95%;
    max-width: none;
    margin: 1rem;
  }

  .features-grid {
    grid-template-columns: 1fr;
    gap: 0.75rem;
  }

  .feature-card {
    padding: 0.75rem;
  }

  .feature-icon {
    width: 35px;
    height: 35px;
    font-size: 1.3rem;
  }

  .about-content {
    padding: 1.5rem;
  }

  .app-info h2 {
    font-size: 1.8rem;
  }

  .version-info {
    padding: 1rem;
  }

  .info-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }

  .info-item .value {
    font-size: 0.85rem;
  }
}

/* 侧边栏链接样式 */
.sidebar-links {
  margin-top: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
}

.about-link-sidebar,
.help-link-sidebar {
  font-size: 0.7rem;
  color: #667eea;
  cursor: pointer;
  text-decoration: none;
  transition: color 0.2s ease;
  user-select: none;
  font-weight: 400;
}

.about-link-sidebar:hover,
.help-link-sidebar:hover {
  color: #764ba2;
}

.separator {
  font-size: 0.7rem;
  color: #999;
  user-select: none;
}

/* AI确认弹窗样式 */
.ai-confirm-popup-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000; /* 比批量处理弹窗更高 */
  animation: fadeIn 0.3s ease;
}

.ai-confirm-popup {
  background: rgba(255, 255, 255, 0.98);
  backdrop-filter: blur(15px);
  border-radius: 16px;
  box-shadow: 0 25px 80px rgba(0, 0, 0, 0.25), 0 10px 30px rgba(0, 0, 0, 0.15);
  width: 90%;
  max-width: 480px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.3);
  animation: slideIn 0.3s ease;
}

.ai-confirm-popup .dialog-header {
  background: linear-gradient(135deg, rgba(23, 162, 184, 0.1) 0%, rgba(0, 123, 255, 0.1) 100%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.ai-confirm-popup .dialog-header h3 {
  color: #1a73e8;
  font-weight: 600;
}

.ai-confirm-popup .dialog-content {
  padding: 1.5rem;
}

.ai-confirm-popup .dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  background: rgba(248, 249, 250, 0.8);
  border-top: 1px solid rgba(255, 255, 255, 0.2);
}

.ai-confirm-popup .btn {
  padding: 0.6rem 1.2rem;
  font-size: 0.9rem;
  border-radius: 8px;
  font-weight: 500;
  min-width: 90px;
}

.ai-confirm-popup .btn-cancel {
  background: rgba(108, 117, 125, 0.1);
  color: #6c757d;
  border: 1px solid rgba(108, 117, 125, 0.2);
}

.ai-confirm-popup .btn-cancel:hover {
  background: rgba(108, 117, 125, 0.2);
  transform: translateY(-1px);
}

.ai-confirm-popup .btn-cache {
  background: linear-gradient(135deg, #28a745 0%, #20c997 100%);
  color: white;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.ai-confirm-popup .btn-cache:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(40, 167, 69, 0.3);
}

.ai-confirm-popup .btn-reprocess {
  background: linear-gradient(135deg, #fd7e14 0%, #ffc107 100%);
  color: #212529;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.ai-confirm-popup .btn-reprocess:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(253, 126, 20, 0.3);
}
</style>
