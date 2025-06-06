<script lang="ts" setup>
import { ref, onMounted, computed, watch } from 'vue'
import PDFViewer from './components/PDFViewer.vue'
import ConfigPanel from './components/ConfigPanel.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import ProgressPanel from './components/ProgressPanel.vue'
import ErrorHandler from './components/ErrorHandler.vue'
import TextEditor from './components/TextEditor.vue'
import { LoadDocument, GetCurrentDocument, ProcessPages, ProcessPagesForce, CheckProcessedPages, GetConfig, GetSupportedFormats, ExportProcessingResults, SaveFileWithDialog, SaveBinaryFileWithDialog, GetAppVersion } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType } from 'docx'

// 响应式数据
const currentDocument = ref<any>(null)
const selectedPages = ref<number[]>([])
const showConfig = ref(false)
const showHistory = ref(false)
const showExportDialog = ref(false)
const exportFormat = ref('txt')
const showTextEditor = ref(false)
const editingPageNumber = ref(0)
const processing = ref(false)
const appVersionInfo = ref<any>(null)

// 编辑器拖拽相关状态
const editorPosition = ref({ x: 50, y: 50 }) // 更靠近左上角，避免遮挡太多内容
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
const showProcessConfirmDialog = ref(false)
const processConfirmData = ref<any>(null)

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

// 生命周期
onMounted(async () => {
  loadLastExportFormat()

  // 加载支持的格式
  try {
    supportedFormats.value = await GetSupportedFormats()
  } catch (error) {
    console.error('获取支持格式失败:', error)
  }

  // 加载版本信息
  try {
    appVersionInfo.value = await GetAppVersion()
    console.log('应用版本信息:', appVersionInfo.value)
  } catch (error) {
    console.error('获取版本信息失败:', error)
  }

  // 监听事件
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
  })

  EventsOn('processing-complete', async (data: any) => {
    processing.value = false
    console.log('处理完成:', data)

    // 强制刷新文档数据，但保持当前页面状态
    try {
      const refreshedDoc = await GetCurrentDocument()
      if (refreshedDoc) {
        currentDocument.value = refreshedDoc
        console.log('文档数据已刷新:', refreshedDoc)

        // 通知 PDFViewer 保持当前页面，不要跳转
        window.dispatchEvent(new CustomEvent('document-refreshed', {
          detail: {
            document: refreshedDoc,
            keepCurrentPage: true,
            processedPages: data.processedPages || []
          }
        }))
      } else {
        currentDocument.value = data.document
      }
    } catch (error) {
      console.error('刷新文档数据失败:', error)
      currentDocument.value = data.document
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
  })

  // 监听历史记录删除事件
  window.addEventListener('history-record-deleted', handleHistoryRecordDeleted)
})

// 监听导出格式变化，实时保存
watch(exportFormat, (newFormat) => {
  saveExportFormat(newFormat)
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

const handleEditPage = (pageNumber: number) => {
  editingPageNumber.value = pageNumber
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

const toggleConfig = () => {
  showConfig.value = !showConfig.value
}

const toggleHistory = () => {
  showHistory.value = !showHistory.value
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
    // 生成默认文件名
    const timestamp = new Date().toISOString().slice(0, 19).replace(/:/g, '-')
    const defaultFileName = `${currentDocument.value?.title || 'PDF处理结果'}_${timestamp}.${exportFormat.value}`

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
        return
      }

      showExportDialog.value = false

      window.dispatchEvent(new CustomEvent('show-success', {
        detail: `导出成功：${filePath}`
      }))
    } else {
      // 其他格式使用后端保存
      const result = await ExportProcessingResults(exportFormat.value)

      const filePath = await SaveFileWithDialog(result, defaultFileName, [
        {
          DisplayName: getFormatDisplayName(exportFormat.value),
          Pattern: `*.${exportFormat.value}`
        }
      ])

      if (!filePath) {
        return
      }

      showExportDialog.value = false

      window.dispatchEvent(new CustomEvent('show-success', {
        detail: `导出成功：${filePath}`
      }))
    }
  } catch (error) {
    console.error('导出失败:', error)
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `导出失败：${error}`
    }))
  }
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

    // 获取所有已处理的页面
    const processedPages = currentDocument.value.pages.filter((page: any) => page.processed)

    if (processedPages.length === 0) {
      throw new Error('没有已处理的页面可以导出')
    }

    // 合并所有页面的文本，使用分页符分隔
    let allText = ''
    for (let i = 0; i < processedPages.length; i++) {
      const page = processedPages[i]
      // 优先使用 OCR 结果，其次是 AI 结果，最后是原生文本
      const text = page.ocr_text || page.ai_text || page.text || ''
      if (text) {
        if (i > 0) {
          allText += '\n\n[PAGE_BREAK]\n\n' // 分页符标记
        }
        allText += text
      }
    }

    // 检测文本中是否包含表格
    const hasTable = detectTable(allText)

    const doc = new Document({
      sections: [{
        properties: {
          page: {
            size: {
              orientation: 'portrait',
            },
          },
        },
        children: [
          ...(hasTable ? generateTableContent(allText) : generateTextContent(allText))
        ],
      }],
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
          @file-select="handleFileSelect"
          @page-select="handlePageSelect"
          @edit-page="handleEditPage"
          @process-pages="(pageNumbers, forceReprocess) => handleProcessPages(pageNumbers, forceReprocess)"
          @page-rendered="handlePageRendered"
          @ai-processing-complete="handleAIProcessingComplete"
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
          <h3>导出处理结果</h3>
          <button @click="showExportDialog = false" class="close-btn">&times;</button>
        </div>

        <div class="dialog-content">
          <div class="format-selection">
            <label>选择导出格式：</label>
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
              <strong>已处理页面数：</strong>
              {{ currentDocument?.pages?.filter((p: any) => p.processed).length || 0 }} 页
            </p>
          </div>
        </div>

        <div class="dialog-actions">
          <button @click="showExportDialog = false" class="btn btn-secondary">
            取消
          </button>
          <button @click="handleExport" class="btn btn-primary">
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

    <!-- 进度面板 -->
    <ProgressPanel v-if="processing" :progress="progress" />

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
}

.sidebar {
  width: 320px;
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
  padding: 1rem;
  border-radius: 4px;
  border-left: 4px solid #007bff;
}

.export-info p {
  margin: 0 0 0.5rem 0;
  color: #666;
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
</style>
