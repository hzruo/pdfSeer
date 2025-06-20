<script lang="ts" setup>
import { ref, onMounted, computed, watch } from 'vue'
import { GetHistoryRecords, GetHistoryPages, SearchHistory, SaveFileWithDialog, SaveBinaryFileWithDialog, GetDocumentHistoryPages, DeleteHistoryRecord } from '../../wailsjs/go/main/App'
import { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType } from 'docx'
import { renderMarkdown, hasMarkdownSyntax } from '../utils/markdown'

// Emits
const emit = defineEmits<{
  'close': []
}>()

// 响应式数据
const loading = ref(false)
const searchQuery = ref('')
const searchResults = ref<any[]>([])
const historyRecords = ref<any[]>([])
const selectedRecord = ref<any>(null)
const selectedPages = ref<any[]>([])
const viewMode = ref<'list' | 'detail'>('list')
const currentPageIndex = ref(0) // 当前页面索引
const pagesPerView = ref(5) // 每次显示的页面数
const showExportDialog = ref(false)
const exportFormat = ref('txt')
const listViewMode = ref<'grouped' | 'detailed'>('grouped') // 列表视图模式
const exportMode = ref<'single' | 'document'>('single') // 导出模式：单个记录或整个文档
const showDeleteDialog = ref(false)
const recordToDelete = ref<any>(null)

// 计算属性
const filteredRecords = computed(() => {
  if (!searchQuery.value) return historyRecords.value
  return searchResults.value
})

// 按文档分组的记录
const groupedRecords = computed(() => {
  const groups = new Map()

  filteredRecords.value.forEach((record: any) => {
    const key = record.document_path
    if (!groups.has(key)) {
      groups.set(key, {
        document_path: record.document_path,
        document_name: record.document_name,
        records: [],
        total_pages: 0,
        latest_date: record.processed_at,
        latest_status: record.status
      })
    }

    const group = groups.get(key)
    group.records.push(record)
    group.total_pages += record.page_count

    // 更新最新的处理时间和状态
    if (new Date(record.processed_at) > new Date(group.latest_date)) {
      group.latest_date = record.processed_at
      group.latest_status = record.status
    }
  })

  return Array.from(groups.values()).sort((a, b) =>
    new Date(b.latest_date).getTime() - new Date(a.latest_date).getTime()
  )
})

// 分页相关计算属性
const totalPages = computed(() => selectedPages.value.length)
const totalPageGroups = computed(() => {
  if (totalPages.value === 0) return 0
  return Math.ceil(totalPages.value / pagesPerView.value)
})
const currentPageGroup = computed(() => {
  if (totalPages.value === 0) return 0

  // 计算当前显示的是第几组
  // 例如：页面索引8，每组5页 → 第2组（索引8-9对应第9-10页，属于第2组）
  // 但如果是最后一组且不满5页，需要特殊处理
  const groupIndex = Math.floor(currentPageIndex.value / pagesPerView.value) + 1

  // 确保不超过总组数
  return Math.min(groupIndex, totalPageGroups.value)
})

const visiblePages = computed(() => {
  const start = currentPageIndex.value
  const end = Math.min(start + pagesPerView.value, totalPages.value)
  return selectedPages.value.slice(start, end)
})

const canGoPrevious = computed(() => currentPageIndex.value > 0)
const canGoNext = computed(() => currentPageIndex.value + pagesPerView.value < totalPages.value)

// 页面范围显示
const pageRangeDisplay = computed(() => {
  if (totalPages.value === 0) return ''

  const startPage = currentPageIndex.value + 1
  const endPage = Math.min(currentPageIndex.value + pagesPerView.value, totalPages.value)

  if (startPage === endPage) {
    return `第 ${startPage} 页`
  } else {
    return `第 ${startPage}-${endPage} 页`
  }
})

const missingPages = computed(() => {
  if (!selectedRecord.value || selectedPages.value.length === 0) return []

  // 获取实际处理的页面号范围
  const pageNumbers = selectedPages.value.map((p: any) => p.page_number)
  const minPage = Math.min(...pageNumbers)
  const maxPage = Math.max(...pageNumbers)

  // 检查在实际处理范围内是否有缺失的页面
  const missing: number[] = []

  // 如果处理的页面是连续的（比如1-5页），检查中间是否有缺失
  // 如果处理的页面是不连续的（比如只处理第4页），则不应该报告缺失
  if (pageNumbers.length > 1) {
    for (let i = minPage; i <= maxPage; i++) {
      if (!pageNumbers.includes(i)) {
        missing.push(i)
      }
    }
  }

  return missing
})

// 从localStorage加载上次的导出格式
const loadLastExportFormat = () => {
  const saved = localStorage.getItem('historyPanel_exportFormat')
  if (saved && ['txt', 'markdown', 'html', 'rtf', 'docx'].includes(saved)) {
    exportFormat.value = saved
  }
}

// 保存导出格式到localStorage
const saveExportFormat = (format: string) => {
  localStorage.setItem('historyPanel_exportFormat', format)
}

// 生命周期
onMounted(async () => {
  loadLastExportFormat()
  await loadHistoryRecords()
})

// 监听导出格式变化，实时保存
watch(exportFormat, (newFormat) => {
  saveExportFormat(newFormat)
})

// 方法
const loadHistoryRecords = async () => {
  try {
    loading.value = true
    const records = await GetHistoryRecords(20) // 获取最近20条记录
    // 强制触发响应式更新
    historyRecords.value = [...(records || [])]
  } catch (error) {
    console.error('加载历史记录失败:', error)
  } finally {
    loading.value = false
  }
}

const searchHistoryRecords = async () => {
  if (!searchQuery.value.trim()) {
    searchResults.value = []
    return
  }

  try {
    const results = await SearchHistory(searchQuery.value, 50)
    searchResults.value = results || []
  } catch (error) {
    console.error('搜索历史记录失败:', error)
  }
}

const selectRecord = async (record: any) => {
  selectedRecord.value = record
  viewMode.value = 'detail'
  currentPageIndex.value = 0 // 重置分页

  try {
    const pages = await GetHistoryPages(record.id)
    // 按页码排序，确保第一页在最前面
    const sortedPages = (pages || []).sort((a: any, b: any) => a.page_number - b.page_number)
    selectedPages.value = sortedPages
  } catch (error) {
    console.error('加载历史页面失败:', error)
  }
}

const backToList = () => {
  viewMode.value = 'list'
  selectedRecord.value = null
  selectedPages.value = []
  currentPageIndex.value = 0
}

// 分页导航方法
const goToPreviousPages = () => {
  if (canGoPrevious.value) {
    currentPageIndex.value = Math.max(0, currentPageIndex.value - pagesPerView.value)
  }
}

const goToNextPages = () => {
  if (canGoNext.value) {
    currentPageIndex.value = currentPageIndex.value + pagesPerView.value
  }
}

const goToPageGroup = (groupIndex: number) => {
  const newIndex = (groupIndex - 1) * pagesPerView.value
  if (newIndex >= 0 && newIndex < totalPages.value) {
    currentPageIndex.value = newIndex
  }
}

const formatDate = (dateStr: string) => {
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN')
}

const formatStatus = (status: string) => {
  const statusMap: Record<string, string> = {
    'processing': '处理中',
    'completed': '已完成',
    'failed': '失败',
    'cancelled': '已取消'
  }
  return statusMap[status] || status
}

const getStatusClass = (status: string) => {
  const classMap: Record<string, string> = {
    'processing': 'status-processing',
    'completed': 'status-completed',
    'failed': 'status-failed',
    'cancelled': 'status-cancelled'
  }
  return classMap[status] || ''
}

const truncateText = (text: string, maxLength: number = 100) => {
  if (!text) return '无内容'
  if (text.length <= maxLength) return text
  return text.substring(0, maxLength) + '...'
}

const close = () => {
  emit('close')
}

// 渲染AI处理结果的markdown
const renderAIProcessedText = (text: string) => {
  if (!text) return ''

  // 检测是否包含markdown语法，如果包含则渲染，否则保持原样
  if (hasMarkdownSyntax(text)) {
    return renderMarkdown(text)
  }

  // 对于纯文本，保持换行并转义HTML
  return text.replace(/&/g, '&amp;')
             .replace(/</g, '&lt;')
             .replace(/>/g, '&gt;')
             .replace(/\n/g, '<br>')
}

// 导出历史记录
const handleExport = async () => {
  try {
    if (!selectedRecord.value) {
      window.dispatchEvent(new CustomEvent('show-warning', {
        detail: '没有选择的记录'
      }))
      return
    }

    let content = ''
    let defaultFileName = ''

    if (exportMode.value === 'document') {
      // 按文档导出所有相关记录
      const timestamp = new Date().toISOString().slice(0, 19).replace(/:/g, '-')
      const docName = selectedRecord.value.document_name || '文档'
      defaultFileName = `${docName}_完整记录_${timestamp}.${exportFormat.value}`

      if (exportFormat.value === 'docx') {
        content = await generateDocumentDocxContent()
      } else {
        content = await generateDocumentExportContent()
      }
    } else {
      // 单个记录导出
      if (selectedPages.value.length === 0) {
        window.dispatchEvent(new CustomEvent('show-warning', {
          detail: '没有可导出的内容'
        }))
        return
      }
      const timestamp = new Date().toISOString().slice(0, 19).replace(/:/g, '-')
      const docName = selectedRecord.value.document_name || '历史记录'
      defaultFileName = `${docName}_历史记录_${timestamp}.${exportFormat.value}`

      if (exportFormat.value === 'docx') {
        content = await generateDocxContent()
      } else {
        content = generateExportContent()
      }
    }

    if (exportFormat.value === 'docx') {
      // 显示生成提示
      window.dispatchEvent(new CustomEvent('show-info', {
        detail: '正在生成DOCX文档，请稍候...'
      }))

      // 使用后端二进制保存对话框
      const filePath = await SaveBinaryFileWithDialog(content, defaultFileName, [
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
      const filePath = await SaveFileWithDialog(content, defaultFileName, [
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

const generateExportContent = () => {
  if (!selectedRecord.value || selectedPages.value.length === 0) {
    return ''
  }

  let content = ''
  const record = selectedRecord.value

  // 添加文档信息头部
  switch (exportFormat.value) {
    case 'markdown':
      content += `# ${record.document_name} - 历史记录\n\n`
      content += `**处理时间:** ${formatDate(record.processed_at)}\n\n`
      content += `**状态:** ${formatStatus(record.status)}\n\n`
      content += `**页数:** ${record.page_count}\n\n`
      if (record.ai_model) content += `**AI模型:** ${record.ai_model}\n\n`
      if (record.cost) content += `**成本:** $${record.cost.toFixed(4)}\n\n`
      content += '---\n\n'
      break
    case 'html':
      content += `<h1>${record.document_name} - 历史记录</h1>\n`
      content += `<p><strong>处理时间:</strong> ${formatDate(record.processed_at)}</p>\n`
      content += `<p><strong>状态:</strong> ${formatStatus(record.status)}</p>\n`
      content += `<p><strong>页数:</strong> ${record.page_count}</p>\n`
      if (record.ai_model) content += `<p><strong>AI模型:</strong> ${record.ai_model}</p>\n`
      if (record.cost) content += `<p><strong>成本:</strong> $${record.cost.toFixed(4)}</p>\n`
      content += '<hr>\n'
      break
    case 'rtf':
      content += '{\\rtf1\\ansi\\ansicpg936\\deff0\\deflang2052\n'
      content += '{\\fonttbl{\\f0\\fswiss\\fcharset134 Microsoft YaHei;}{\\f1\\fmodern\\fcharset0 Courier New;}}\n'
      content += '{\\colortbl;\\red0\\green0\\blue0;\\red0\\green0\\blue255;}\n'
      content += `\\viewkind4\\uc1\\pard\\cf1\\lang2052\\f0\\fs28\\b ${record.document_name} - 历史记录\\par\n`
      content += '\\par\n'
      content += `\\cf0\\fs22\\b0\\f1 处理时间: ${formatDate(record.processed_at)}\\par\n`
      content += `状态: ${formatStatus(record.status)}\\par\n`
      content += `页数: ${record.page_count}\\par\n`
      if (record.ai_model) content += `AI模型: ${record.ai_model}\\par\n`
      if (record.cost) content += `成本: $${record.cost.toFixed(4)}\\par\n`
      content += '\\par\n'
      break
    default: // txt
      content += `${record.document_name} - 历史记录\n`
      content += `处理时间: ${formatDate(record.processed_at)}\n`
      content += `状态: ${formatStatus(record.status)}\n`
      content += `页数: ${record.page_count}\n`
      if (record.ai_model) content += `AI模型: ${record.ai_model}\n`
      if (record.cost) content += `成本: $${record.cost.toFixed(4)}\n`
      content += '=' + '='.repeat(50) + '\n\n'
  }

  // 导出所有页面内容
  for (const page of selectedPages.value) {
    // 优先使用 OCR 结果，其次是原始文本
    const text = page.ocr_text || page.original_text || page.ai_processed_text || ''

    if (text) {
      switch (exportFormat.value) {
        case 'markdown':
          content += `## 第 ${page.page_number} 页\n\n`
          content += `${text}\n\n`
          break
        case 'html':
          content += `<h2>第 ${page.page_number} 页</h2>\n`
          content += `<div class="page-content">${text.replace(/\n/g, '<br>\n')}</div>\n\n`
          break
        case 'rtf':
          content += generateRtfPageContent(page.page_number, text)
          break
        default: // txt
          content += `=== 第 ${page.page_number} 页 ===\n`
          content += `${text}\n\n`
      }
    }
  }

  // 如果是RTF格式，添加结束标记
  if (exportFormat.value === 'rtf') {
    content += '}'
  }

  return content
}

const generateDocumentExportContent = async () => {
  if (!selectedRecord.value) return ''

  try {
    // 获取文档的所有页面数据
    const allPages = await GetDocumentHistoryPages(selectedRecord.value.document_path)

    // 按页码排序并去重（如果同一页有多个版本，使用最新的）
    const pageMap = new Map()
    allPages.forEach((page: any) => {
      const existing = pageMap.get(page.page_number)
      if (!existing || new Date(page.created_at) > new Date(existing.created_at)) {
        pageMap.set(page.page_number, page)
      }
    })

    const sortedPages = Array.from(pageMap.values()).sort((a: any, b: any) => a.page_number - b.page_number)

    let content = ''
    const record = selectedRecord.value

    // 添加文档信息头部
    switch (exportFormat.value) {
      case 'markdown':
        content += `# ${record.document_name} - 完整文档\n\n`
        content += `**文件路径:** ${record.document_path}\n\n`
        content += `**总页数:** ${sortedPages.length}\n\n`
        content += `**处理记录数:** ${historyRecords.value.filter((r: any) => r.document_path === record.document_path).length}\n\n`
        content += '---\n\n'
        break
      case 'html':
        content += `<h1>${record.document_name} - 完整文档</h1>\n`
        content += `<p><strong>文件路径:</strong> ${record.document_path}</p>\n`
        content += `<p><strong>总页数:</strong> ${sortedPages.length}</p>\n`
        content += `<p><strong>处理记录数:</strong> ${historyRecords.value.filter((r: any) => r.document_path === record.document_path).length}</p>\n`
        content += '<hr>\n'
        break
      case 'rtf':
        content += '{\\rtf1\\ansi\\ansicpg936\\deff0\\deflang2052\n'
        content += '{\\fonttbl{\\f0\\fswiss\\fcharset134 Microsoft YaHei;}{\\f1\\fmodern\\fcharset0 Courier New;}}\n'
        content += '{\\colortbl;\\red0\\green0\\blue0;\\red0\\green0\\blue255;}\n'
        content += `\\viewkind4\\uc1\\pard\\cf1\\lang2052\\f0\\fs28\\b ${record.document_name} - 完整文档\\par\n`
        content += '\\par\n'
        content += `\\cf0\\fs22\\b0\\f1 文件路径: ${record.document_path}\\par\n`
        content += `总页数: ${sortedPages.length}\\par\n`
        content += `处理记录数: ${historyRecords.value.filter((r: any) => r.document_path === record.document_path).length}\\par\n`
        content += '\\par\n'
        break
      default: // txt
        content += `${record.document_name} - 完整文档\n`
        content += `文件路径: ${record.document_path}\n`
        content += `总页数: ${sortedPages.length}\n`
        content += `处理记录数: ${historyRecords.value.filter((r: any) => r.document_path === record.document_path).length}\n`
        content += '=' + '='.repeat(50) + '\n\n'
    }

    // 导出所有页面内容
    for (const page of sortedPages) {
      const text = page.ocr_text || page.original_text || page.ai_processed_text || ''

      if (text) {
        switch (exportFormat.value) {
          case 'markdown':
            content += `## 第 ${page.page_number} 页\n\n`
            content += `${text}\n\n`
            break
          case 'html':
            content += `<h2>第 ${page.page_number} 页</h2>\n`
            content += `<div class="page-content">${text.replace(/\n/g, '<br>\n')}</div>\n\n`
            break
          case 'rtf':
            content += generateRtfPageContent(page.page_number, text)
            break
          default: // txt
            content += `=== 第 ${page.page_number} 页 ===\n`
            content += `${text}\n\n`
        }
      }
    }

    // 如果是RTF格式，添加结束标记
    if (exportFormat.value === 'rtf') {
      content += '}'
    }

    return content
  } catch (error) {
    console.error('生成文档导出内容失败:', error)
    throw error
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

// 生成RTF页面内容
const generateRtfPageContent = (pageNumber: number, text: string) => {
  return `\\par\\b 第 ${pageNumber} 页\\b0\\par\\par${text.replace(/\\/g, '\\\\').replace(/\{/g, '\\{').replace(/\}/g, '\\}').replace(/\n/g, '\\par\n')}\\par\\par`
}

// 生成DOCX内容（单个记录）
const generateDocxContent = async (): Promise<string> => {
  try {
    if (!selectedRecord.value || selectedPages.value.length === 0) {
      throw new Error('没有可导出的内容')
    }

    // 合并所有页面的文本，使用分页符分隔
    let allText = ''
    for (let i = 0; i < selectedPages.value.length; i++) {
      const page = selectedPages.value[i]
      const text = page.ocr_text || page.original_text || page.ai_processed_text || ''
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

// 生成DOCX内容（文档模式）
const generateDocumentDocxContent = async (): Promise<string> => {
  try {
    if (!selectedRecord.value) {
      throw new Error('没有选择的记录')
    }

    // 获取文档的所有页面数据
    const allPages = await GetDocumentHistoryPages(selectedRecord.value.document_path)

    // 按页码排序并去重
    const pageMap = new Map()
    allPages.forEach((page: any) => {
      const existing = pageMap.get(page.page_number)
      if (!existing || new Date(page.created_at) > new Date(existing.created_at)) {
        pageMap.set(page.page_number, page)
      }
    })

    const sortedPages = Array.from(pageMap.values()).sort((a: any, b: any) => a.page_number - b.page_number)

    // 合并所有页面的文本，使用分页符分隔
    let allText = ''
    for (let i = 0; i < sortedPages.length; i++) {
      const page = sortedPages[i]
      const text = page.ocr_text || page.original_text || page.ai_processed_text || ''
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
  const lines = text.split('\n')
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

// 删除历史记录
const handleDeleteRecord = (record: any) => {
  recordToDelete.value = record
  showDeleteDialog.value = true
}

// 确认删除
const confirmDelete = async () => {
  if (!recordToDelete.value) return

  const record = recordToDelete.value

  try {
    await DeleteHistoryRecord(record.id)

    // 手动从列表中移除记录（立即更新UI）
    const recordIndex = historyRecords.value.findIndex((r: any) => r.id === record.id)
    if (recordIndex !== -1) {
      historyRecords.value.splice(recordIndex, 1)
    }

    // 同时清理搜索结果
    if (searchResults.value.length > 0) {
      const searchIndex = searchResults.value.findIndex((r: any) => r.id === record.id)
      if (searchIndex !== -1) {
        searchResults.value.splice(searchIndex, 1)
      }
    }

    // 如果当前正在查看被删除的记录，返回列表
    if (selectedRecord.value && selectedRecord.value.id === record.id) {
      backToList()
    }

    // 通知主应用可能需要刷新当前文档（如果删除的是当前文档的历史记录）
    window.dispatchEvent(new CustomEvent('history-record-deleted', {
      detail: {
        recordId: record.id,
        documentPath: record.document_path,
        documentName: record.document_name
      }
    }))

    // 刷新历史记录列表（确保数据同步）
    setTimeout(async () => {
      await loadHistoryRecords()
    }, 100)

    window.dispatchEvent(new CustomEvent('show-success', {
      detail: '删除成功，已清理相关数据'
    }))
  } catch (error) {
    console.error('删除失败:', error)
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `删除失败：${error}`
    }))
  } finally {
    // 关闭对话框并清理状态
    showDeleteDialog.value = false
    recordToDelete.value = null
  }
}

// 取消删除
const cancelDelete = () => {
  showDeleteDialog.value = false
  recordToDelete.value = null
}

// 防抖搜索
let searchTimeout: ReturnType<typeof setTimeout> | null = null
const debouncedSearch = () => {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(searchHistoryRecords, 300)
}
</script>

<template>
  <div class="history-overlay">
    <div class="history-panel">
      <!-- 头部 -->
      <div class="panel-header">
        <div class="header-left">
          <button v-if="viewMode === 'detail'" @click="backToList" class="back-btn">
            ← 返回
          </button>
          <h2>{{ viewMode === 'detail' ? '历史详情' : '历史记录' }}</h2>
        </div>
        <button @click="close" class="close-btn">×</button>
      </div>

      <!-- 搜索栏 -->
      <div v-if="viewMode === 'list'" class="search-bar">
        <input 
          v-model="searchQuery"
          @input="debouncedSearch"
          type="text" 
          placeholder="搜索历史记录..."
          class="search-input"
        />
        <button v-if="searchQuery" @click="searchQuery = ''; searchResults = []" class="clear-search">
          ×
        </button>
      </div>

      <!-- 内容 -->
      <div class="panel-content">
        <!-- 列表视图 -->
        <div v-if="viewMode === 'list'" class="list-view">
          <div v-if="loading" class="loading-state">
            <div class="spinner"></div>
            <p>加载中...</p>
          </div>

          <div v-else-if="filteredRecords.length === 0" class="empty-state">
            <div class="empty-icon">📄</div>
            <h3>{{ searchQuery ? '未找到相关记录' : '暂无历史记录' }}</h3>
            <p>{{ searchQuery ? '尝试使用其他关键词搜索' : '开始处理PDF文档后，历史记录将显示在这里' }}</p>
          </div>

          <div v-else class="records-list">
            <div 
              v-for="record in filteredRecords" 
              :key="record.id || record.history_id"
              class="record-item"
              @click="selectRecord(record)"
            >
              <div class="record-header">
                <div class="record-title">
                  {{ record.document_name || record.document_path?.split('/').pop() || '未知文档' }}
                </div>
                <div class="record-actions">
                  <div class="record-status" :class="getStatusClass(record.status)">
                    {{ formatStatus(record.status) }}
                  </div>
                  <button
                    @click.stop="handleDeleteRecord(record)"
                    class="delete-btn"
                    title="删除记录"
                  >
                    🗑️
                  </button>
                </div>
              </div>
              
              <div class="record-meta">
                <span class="record-date">{{ formatDate(record.processed_at) }}</span>
                <span class="record-pages">{{ record.page_count || 1 }} 页</span>
                <span v-if="record.ai_model" class="record-model">{{ record.ai_model }}</span>
              </div>

              <!-- 搜索结果显示片段 -->
              <div v-if="record.snippet" class="record-snippet" v-html="record.snippet"></div>
              
              <div v-if="record.cost" class="record-cost">
                成本: ${{ record.cost.toFixed(4) }}
              </div>
            </div>
          </div>
        </div>

        <!-- 详情视图 -->
        <div v-else-if="viewMode === 'detail'" class="detail-view">
          <div v-if="selectedRecord" class="record-detail">
            <!-- 记录信息 -->
            <div class="detail-header">
              <h3>{{ selectedRecord.document_name }}</h3>
              <div class="detail-meta">
                <div class="meta-item">
                  <strong>处理时间:</strong> {{ formatDate(selectedRecord.processed_at) }}
                </div>
                <div class="meta-item">
                  <strong>状态:</strong> 
                  <span :class="getStatusClass(selectedRecord.status)">
                    {{ formatStatus(selectedRecord.status) }}
                  </span>
                </div>
                <div class="meta-item">
                  <strong>页数:</strong> {{ selectedRecord.page_count }}
                </div>
                <div v-if="selectedRecord.ai_model" class="meta-item">
                  <strong>AI模型:</strong> {{ selectedRecord.ai_model }}
                </div>
                <div v-if="selectedRecord.cost" class="meta-item">
                  <strong>成本:</strong> ${{ selectedRecord.cost.toFixed(4) }}
                </div>
              </div>
            </div>

            <!-- 页面列表 -->
            <div class="pages-section">
              <div class="pages-header">
                <div class="pages-header-left">
                  <h4>页面内容</h4>
                  <div v-if="totalPages > 0" class="pages-info">
                    共 {{ totalPages }} 页，当前显示{{ pageRangeDisplay }}
                  </div>
                  <div v-if="missingPages.length > 0" class="missing-pages-warning">
                    ⚠️ 缺失页面：{{ missingPages.join(', ') }}
                  </div>
                </div>
                <div class="pages-header-right">
                  <button
                    @click="showExportDialog = true"
                    :disabled="selectedPages.length === 0"
                    class="export-btn"
                  >
                    导出历史记录
                  </button>
                </div>
              </div>

              <!-- 分页控制 -->
              <div v-if="totalPages > pagesPerView" class="pagination-controls">
                <button
                  @click="goToPreviousPages"
                  :disabled="!canGoPrevious"
                  class="pagination-btn"
                >
                  ← 上一组
                </button>

                <div class="pagination-info">
                  第 {{ currentPageGroup }} / {{ totalPageGroups }} 组
                </div>

                <button
                  @click="goToNextPages"
                  :disabled="!canGoNext"
                  class="pagination-btn"
                >
                  下一组 →
                </button>

                <div class="pages-per-view-control">
                  <label>每组显示:</label>
                  <select v-model="pagesPerView" class="pages-select">
                    <option :value="3">3页</option>
                    <option :value="5">5页</option>
                    <option :value="10">10页</option>
                  </select>
                </div>
              </div>

              <div v-if="selectedPages.length === 0" class="empty-pages">
                <p>暂无页面数据</p>
              </div>
              <div v-else class="pages-list">
                <div
                  v-for="page in visiblePages"
                  :key="page.id"
                  class="page-item"
                >
                  <div class="page-header">
                    <h5>第 {{ page.page_number }} 页</h5>
                    <div class="page-meta">
                      <span v-if="page.processing_time">
                        处理时间: {{ page.processing_time.toFixed(2) }}s
                      </span>
                    </div>
                  </div>

                  <div class="page-content">
                    <!-- 原始文本 -->
                    <div v-if="page.original_text" class="text-section">
                      <h6>原始文本:</h6>
                      <div class="text-content">{{ page.original_text }}</div>
                    </div>

                    <!-- OCR文本 -->
                    <div v-if="page.ocr_text" class="text-section">
                      <h6>OCR识别:</h6>
                      <div class="text-content">{{ page.ocr_text }}</div>
                    </div>

                    <!-- AI处理文本 -->
                    <div v-if="page.ai_processed_text" class="text-section">
                      <h6>AI处理:</h6>
                      <div class="markdown-content" v-html="renderAIProcessedText(page.ai_processed_text)"></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 导出对话框 -->
      <div v-if="showExportDialog" class="export-dialog-overlay">
        <div class="export-dialog">
          <div class="dialog-header">
            <h3>导出历史记录</h3>
            <button @click="showExportDialog = false" class="close-btn">&times;</button>
          </div>

          <div class="dialog-content">
            <div class="export-mode-selection">
              <label>选择导出范围：</label>
              <div class="mode-description">
                <p v-if="exportMode === 'single'">只导出当前查看的历史记录</p>
                <p v-else-if="exportMode === 'document'">导出该文档的所有历史记录（自动合并去重）</p>
                <p v-else>请选择导出范围</p>
              </div>
              <div class="mode-options">
                <label class="mode-option">
                  <input type="radio" v-model="exportMode" value="single" />
                  <span>当前记录</span>
                </label>
                <label class="mode-option">
                  <input type="radio" v-model="exportMode" value="document" />
                  <span>整个文档</span>
                </label>
              </div>
            </div>

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

            <div class="export-info" v-if="selectedPages.length > 0 && exportMode === 'single'">
              <p>
                <strong>页面数：</strong> {{ selectedPages.length }} 页
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

      <!-- 删除确认对话框 -->
      <div v-if="showDeleteDialog" class="delete-dialog-overlay">
        <div class="delete-dialog">
          <div class="dialog-header">
            <h3>确认删除</h3>
          </div>

          <div class="dialog-content">
            <div class="warning-icon">⚠️</div>
            <p class="warning-text">
              确定要删除记录 <strong>"{{ recordToDelete?.document_name || '未知文档' }}"</strong> 吗？
            </p>
            <p class="warning-note">
              此操作不可撤销！删除后将无法恢复该记录的所有数据。
            </p>
          </div>

          <div class="dialog-actions">
            <button @click="cancelDelete" class="btn btn-secondary">
              取消
            </button>
            <button @click="confirmDelete" class="btn btn-danger">
              确定删除
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.history-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.history-panel {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15), 0 8px 20px rgba(0, 0, 0, 0.1);
  width: 90%;
  max-width: 1000px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  border: 1px solid rgba(255, 255, 255, 0.2);
  animation: slideIn 0.3s ease;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.1) 0%, rgba(118, 75, 162, 0.1) 100%);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.back-btn {
  background: rgba(102, 126, 234, 0.1);
  border: 1px solid rgba(102, 126, 234, 0.2);
  color: #667eea;
  cursor: pointer;
  font-size: 0.9rem;
  padding: 0.5rem 1rem;
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.back-btn:hover {
  background: rgba(102, 126, 234, 0.2);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.2);
}

.panel-header h2 {
  margin: 0;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.panel-header h2::before {
  content: '📚';
  font-size: 1.2rem;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: #666;
  padding: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
}

.close-btn:hover {
  background: #f0f0f0;
}

.search-bar {
  padding: 1rem 2rem;
  border-bottom: 1px solid #e0e0e0;
  position: relative;
}

.search-input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9rem;
}

.search-input:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.clear-search {
  position: absolute;
  right: 3rem;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: #666;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 0.25rem;
}

.panel-content {
  flex: 1;
  overflow-y: auto;
}

.loading-state,
.empty-state {
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

.empty-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.empty-state h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.records-list {
  padding: 1rem;
}

.record-item {
  background: #f8f9fa;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 1rem;
  cursor: pointer;
  transition: background-color 0.2s, box-shadow 0.2s;
}

.record-item:hover {
  background: #e9ecef;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.record-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.record-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.delete-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.25rem;
  border-radius: 4px;
  font-size: 0.9rem;
  opacity: 0.7;
  transition: all 0.2s;
}

.delete-btn:hover {
  opacity: 1;
  background: #fee;
  transform: scale(1.1);
}

.record-title {
  font-weight: 500;
  color: #333;
  font-size: 1rem;
}

.record-status {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8rem;
  font-weight: 500;
}

.status-processing {
  background: #fff3cd;
  color: #856404;
}

.status-completed {
  background: #d4edda;
  color: #155724;
}

.status-failed {
  background: #f8d7da;
  color: #721c24;
}

.status-cancelled {
  background: #fff3cd;
  color: #856404;
}

.record-meta {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  color: #666;
  margin-bottom: 0.5rem;
}

.record-snippet {
  font-size: 0.85rem;
  color: #555;
  line-height: 1.4;
  margin-bottom: 0.5rem;
}

.record-snippet :deep(mark) {
  background: #ffeb3b;
  padding: 0.1rem 0.2rem;
  border-radius: 2px;
}

.record-cost {
  font-size: 0.8rem;
  color: #28a745;
  font-weight: 500;
}

.detail-view {
  padding: 2rem;
}

.detail-header {
  margin-bottom: 2rem;
}

.detail-header h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.detail-meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.meta-item {
  font-size: 0.9rem;
}

.meta-item strong {
  color: #333;
}

.pages-section h4 {
  margin: 0 0 1rem 0;
  color: #333;
}

.empty-pages {
  text-align: center;
  padding: 2rem;
  color: #666;
}

.pages-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.page-item {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  overflow: hidden;
}

.page-header {
  background: #f8f9fa;
  padding: 1rem;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-header h5 {
  margin: 0;
  color: #333;
}

.page-meta {
  font-size: 0.8rem;
  color: #666;
}

.page-content {
  padding: 1rem;
}

.text-section {
  margin-bottom: 1rem;
}

.text-section:last-child {
  margin-bottom: 0;
}

.text-section h6 {
  margin: 0 0 0.5rem 0;
  color: #333;
  font-size: 0.9rem;
}

.text-content {
  background: #f8f9fa;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  padding: 0.75rem;
  font-size: 0.85rem;
  line-height: 1.4;
  white-space: pre-wrap;
  max-height: 200px;
  overflow-y: auto;
}

/* Markdown内容样式覆盖 */
.markdown-content {
  white-space: normal !important;
  background: #f8f9fa;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  padding: 0.75rem;
  font-size: 0.85rem;
  line-height: 1.4;
  max-height: 200px;
  overflow-y: auto;
  word-wrap: break-word;
  overflow-wrap: break-word;
}

.markdown-content h1,
.markdown-content h2,
.markdown-content h3,
.markdown-content h4,
.markdown-content h5,
.markdown-content h6 {
  margin: 0.8em 0 0.3em 0;
  font-weight: 600;
  line-height: 1.3;
  color: #2c3e50;
}

.markdown-content h1 { font-size: 1.4em; }
.markdown-content h2 { font-size: 1.2em; }
.markdown-content h3 { font-size: 1.1em; }
.markdown-content h4 { font-size: 1em; }
.markdown-content h5,
.markdown-content h6 { font-size: 0.9em; }

.markdown-content p {
  margin: 0.5em 0;
  line-height: 1.4;
}

.markdown-content ul,
.markdown-content ol {
  margin: 0.5em 0;
  padding-left: 1.5em;
}

.markdown-content li {
  margin: 0.2em 0;
  line-height: 1.3;
}

.markdown-content code {
  background: #e9ecef;
  padding: 0.1em 0.3em;
  border-radius: 3px;
  font-family: 'Courier New', monospace;
  font-size: 0.8em;
  color: #d63384;
}

.markdown-content pre {
  background: #e9ecef;
  border: 1px solid #dee2e6;
  border-radius: 4px;
  padding: 0.5em;
  overflow-x: auto;
  margin: 0.5em 0;
}

.markdown-content pre code {
  background: none;
  padding: 0;
  color: #333;
}

.markdown-content blockquote {
  margin: 0.5em 0;
  padding: 0.3em 0.5em;
  border-left: 3px solid #ddd;
  background: #f1f3f4;
  color: #666;
  font-style: italic;
}

.markdown-content strong {
  font-weight: 600;
  color: #2c3e50;
}

.markdown-content em {
  font-style: italic;
  color: #555;
}

/* 页面列表样式 */
.pages-section {
  margin-top: 2rem;
}

.pages-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.pages-header h4 {
  margin: 0;
  color: #333;
}

.pages-info {
  font-size: 0.9rem;
  color: #666;
}

.missing-pages-warning {
  color: #e74c3c;
  font-size: 0.85rem;
  background: #fdf2f2;
  padding: 0.5rem;
  border-radius: 4px;
  border-left: 3px solid #e74c3c;
  margin-top: 0.5rem;
}

/* 分页控制样式 */
.pagination-controls {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.pagination-btn {
  padding: 0.5rem 1rem;
  border: 1px solid #007bff;
  background: white;
  color: #007bff;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
}

.pagination-btn:hover:not(:disabled) {
  background: #007bff;
  color: white;
}

.pagination-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  border-color: #ccc;
  color: #ccc;
}

.pagination-info {
  font-size: 0.9rem;
  color: #666;
  font-weight: 500;
}

.pages-per-view-control {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-left: auto;
}

.pages-per-view-control label {
  font-size: 0.9rem;
  color: #666;
  margin: 0;
}

.pages-select {
  padding: 0.25rem 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9rem;
}

/* 页面头部样式 */
.pages-header-left {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.export-btn {
  background: #28a745;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s;
}

.export-btn:hover:not(:disabled) {
  background: #218838;
}

.export-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

/* 导出对话框样式 */
.export-dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1600;
  padding: 1rem;
}

.export-dialog {
  background: rgba(255, 255, 255, 0.98);
  backdrop-filter: blur(15px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  border: 1px solid rgba(255, 255, 255, 0.2);
  width: 100%;
  max-width: 450px;
  max-height: 85vh;
  overflow: hidden;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.25rem;
  background: rgba(248, 249, 250, 0.95);
  backdrop-filter: blur(15px);
  border-bottom: 1px solid rgba(224, 224, 224, 0.3);
}

.dialog-header h3 {
  margin: 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
}

.dialog-content {
  padding: 1.25rem;
  max-height: 60vh;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.dialog-content::-webkit-scrollbar {
  width: 6px;
}

.dialog-content::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 3px;
}

.dialog-content::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 3px;
}

.dialog-content::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.export-mode-selection {
  margin-bottom: 1rem;
}

.export-mode-selection > label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #333;
  font-size: 0.95rem;
}

.mode-description {
  margin-bottom: 0.75rem;
  padding: 0.5rem 0.75rem;
  background: rgba(102, 126, 234, 0.05);
  border-radius: 6px;
  border-left: 3px solid #667eea;
  min-height: 1.5rem;
  display: flex;
  align-items: center;
}

.mode-description p {
  margin: 0;
  color: #666;
  font-size: 0.85rem;
  line-height: 1.3;
}

.mode-options {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
}

.mode-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem;
  border: 2px solid rgba(224, 224, 224, 0.5);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: rgba(255, 255, 255, 0.8);
  justify-content: center;
}

.mode-option:hover {
  border-color: rgba(102, 126, 234, 0.6);
  background: rgba(102, 126, 234, 0.05);
}

.mode-option:has(input[type="radio"]:checked) {
  border-color: #667eea;
  background: rgba(102, 126, 234, 0.1);
}

.mode-option input[type="radio"] {
  margin: 0;
  accent-color: #667eea;
  flex-shrink: 0;
}

.mode-option span {
  font-weight: 500;
  font-size: 0.9rem;
  white-space: nowrap;
}

.format-selection label {
  display: block;
  margin-bottom: 0.75rem;
  font-weight: 500;
  color: #333;
  font-size: 0.95rem;
}

.format-options {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.format-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem;
  border: 2px solid rgba(224, 224, 224, 0.5);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
  background: rgba(255, 255, 255, 0.8);
}

.format-option:hover {
  border-color: rgba(102, 126, 234, 0.6);
  background: rgba(102, 126, 234, 0.05);
}

.format-option input[type="radio"] {
  margin: 0;
  flex-shrink: 0;
  accent-color: #667eea;
}

.format-option input[type="radio"]:checked + .option-content {
  color: #667eea;
}

.format-option:has(input[type="radio"]:checked) {
  border-color: #667eea;
  background: rgba(102, 126, 234, 0.1);
}

.option-content {
  flex: 1;
  min-width: 0;
}

.option-title {
  font-weight: 500;
  font-size: 0.85rem;
  overflow: hidden;
  text-overflow: ellipsis;
}

.export-info {
  background: rgba(40, 167, 69, 0.1);
  padding: 0.75rem;
  border-radius: 8px;
  border-left: 3px solid #28a745;
}

.export-info p {
  margin: 0 0 0.25rem 0;
  color: #666;
  font-size: 0.85rem;
  line-height: 1.3;
}

.export-info p:last-child {
  margin-bottom: 0;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  background: rgba(248, 249, 250, 0.95);
  backdrop-filter: blur(15px);
  border-top: 1px solid rgba(224, 224, 224, 0.3);
}

.btn {
  padding: 0.75rem 1.25rem;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 500;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
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

.btn-primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

.btn-secondary {
  background: linear-gradient(135deg, #6c757d 0%, #495057 100%);
  color: white;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.btn-secondary:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(108, 117, 125, 0.4);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .export-dialog {
    width: 95%;
    max-width: none;
    margin: 0.5rem;
  }

  .mode-options,
  .format-options {
    grid-template-columns: 1fr;
    gap: 0.5rem;
  }

  .dialog-header,
  .dialog-content,
  .dialog-actions {
    padding: 1rem;
  }

  .dialog-content {
    max-height: 50vh;
  }

  .export-mode-selection,
  .format-options {
    margin-bottom: 0.75rem;
  }
}

@media (max-height: 700px) {
  .export-dialog {
    max-height: 90vh;
  }

  .dialog-content {
    max-height: 45vh;
  }

  .mode-option,
  .format-option {
    padding: 0.5rem;
  }

  .export-info {
    padding: 0.5rem;
  }

  .export-info p {
    font-size: 0.8rem;
  }
}

/* 删除确认对话框样式 */
.delete-dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1700;
}

.delete-dialog {
  background: white;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  width: 90%;
  max-width: 400px;
  overflow: hidden;
}

.delete-dialog .dialog-header {
  padding: 1rem 1.5rem;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
}

.delete-dialog .dialog-header h3 {
  margin: 0;
  color: #dc3545;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.delete-dialog .dialog-content {
  padding: 1.5rem;
  text-align: center;
}

.warning-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.warning-text {
  font-size: 1.1rem;
  margin-bottom: 1rem;
  color: #333;
}

.warning-note {
  font-size: 0.9rem;
  color: #666;
  margin-bottom: 0;
}

.delete-dialog .dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  padding: 1rem 1.5rem;
  background: #f8f9fa;
  border-top: 1px solid #e0e0e0;
}

.btn-danger {
  background: #dc3545;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s;
}

.btn-danger:hover {
  background: #c82333;
}
</style>
