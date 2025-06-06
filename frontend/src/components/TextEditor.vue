<script lang="ts" setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { UpdatePageText, SaveFileWithDialog, SaveBinaryFileWithDialog } from '../../wailsjs/go/main/App'
import { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType } from 'docx'
import { saveAs } from 'file-saver'

// Props
interface Props {
  pageNumber: number
  originalText?: string
  ocrText?: string
  aiText?: string
  readonly?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false
})

// Emits
const emit = defineEmits<{
  'text-updated': [pageNumber: number, textType: string, text: string]
  'close': []
}>()

// 响应式数据
const activeTab = ref<'original' | 'ocr' | 'ai'>('ocr')
const editingText = ref('')
const isEditing = ref(false)
const saving = ref(false)
const hasChanges = ref(false)
const showExportDialog = ref(false)
const exportFormat = ref('txt')

// 拖拽相关状态
const isDragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })
const position = ref({ x: 100, y: 100 }) // 初始位置

// 拉伸相关状态
const isResizing = ref(false)
const resizeDirection = ref('')
const size = ref({ width: 800, height: 600 }) // 初始大小
const minSize = { width: 400, height: 300 } // 最小尺寸

// 从localStorage加载上次的导出格式
const loadLastExportFormat = () => {
  const saved = localStorage.getItem('textEditor_exportFormat')
  if (saved && ['txt', 'markdown', 'html', 'rtf', 'docx'].includes(saved)) {
    exportFormat.value = saved
  }
}

// 保存导出格式到localStorage
const saveExportFormat = (format: string) => {
  localStorage.setItem('textEditor_exportFormat', format)
}

// 计算居中位置
const centerWindow = () => {
  const windowWidth = window.innerWidth
  const windowHeight = window.innerHeight
  const modalWidth = size.value.width
  const modalHeight = size.value.height

  position.value = {
    x: Math.max(0, (windowWidth - modalWidth) / 2),
    y: Math.max(0, (windowHeight - modalHeight) / 2)
  }
}

// 组件挂载时加载上次的导出格式并居中显示
onMounted(() => {
  loadLastExportFormat()
  centerWindow()
})

// 监听导出格式变化，实时保存
watch(exportFormat, (newFormat) => {
  saveExportFormat(newFormat)
})

// 计算属性
const currentText = computed(() => {
  switch (activeTab.value) {
    case 'original':
      return props.originalText || '无原始文本'
    case 'ocr':
      return props.ocrText || '无OCR文本'
    case 'ai':
      return props.aiText || '无AI处理文本'
    default:
      return ''
  }
})

const canEdit = computed(() => {
  return !props.readonly && (activeTab.value === 'ocr' || activeTab.value === 'ai')
})

const wordCount = computed(() => {
  return editingText.value.length
})

const lineCount = computed(() => {
  return editingText.value.split('\n').length
})

// 监听器
watch(() => props.ocrText, (newText) => {
  if (activeTab.value === 'ocr' && !isEditing.value) {
    editingText.value = newText || ''
  }
}, { immediate: true })

watch(() => props.aiText, (newText) => {
  if (activeTab.value === 'ai' && !isEditing.value) {
    editingText.value = newText || ''
  }
}, { immediate: true })

watch(activeTab, (newTab) => {
  if (!isEditing.value) {
    switch (newTab) {
      case 'ocr':
        editingText.value = props.ocrText || ''
        break
      case 'ai':
        editingText.value = props.aiText || ''
        break
      default:
        editingText.value = ''
    }
  }
  hasChanges.value = false
})

watch(editingText, () => {
  if (isEditing.value) {
    hasChanges.value = editingText.value !== currentText.value
  }
})

// 方法
const startEditing = () => {
  if (!canEdit.value) return
  
  isEditing.value = true
  editingText.value = currentText.value
  hasChanges.value = false
}

// 自定义确认对话框状态
const showConfirmDialog = ref(false)
const confirmMessage = ref('')
const confirmCallback = ref<(() => void) | null>(null)

const cancelEditing = () => {
  if (hasChanges.value) {
    // 显示自定义确认对话框
    confirmMessage.value = '有未保存的更改，确定要取消吗？'
    confirmCallback.value = performCancel
    showConfirmDialog.value = true
  } else {
    performCancel()
  }
}

const performCancel = () => {
  isEditing.value = false
  editingText.value = currentText.value
  hasChanges.value = false
  showConfirmDialog.value = false
}

// 确认对话框处理
const handleConfirm = () => {
  if (confirmCallback.value) {
    confirmCallback.value()
  }
  showConfirmDialog.value = false
}

const handleCancel = () => {
  showConfirmDialog.value = false
  confirmCallback.value = null
}

const saveChanges = async () => {
  if (!hasChanges.value || !canEdit.value) return
  
  try {
    saving.value = true
    
    // 调用后端API更新文本
    await UpdatePageText(props.pageNumber, activeTab.value, editingText.value)
    
    // 通知父组件
    emit('text-updated', props.pageNumber, activeTab.value, editingText.value)
    
    isEditing.value = false
    hasChanges.value = false
    
    // 显示成功消息
    window.dispatchEvent(new CustomEvent('show-success', {
      detail: '文本更新成功'
    }))
    
  } catch (error) {
    console.error('保存文本失败:', error)
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: '保存文本失败: ' + error
    }))
  } finally {
    saving.value = false
  }
}

const copyText = () => {
  const textToCopy = isEditing.value ? editingText.value : currentText.value
  
  if (navigator.clipboard) {
    navigator.clipboard.writeText(textToCopy).then(() => {
      window.dispatchEvent(new CustomEvent('show-success', {
        detail: '文本已复制到剪贴板'
      }))
    }).catch(() => {
      fallbackCopy(textToCopy)
    })
  } else {
    fallbackCopy(textToCopy)
  }
}

const fallbackCopy = (text: string) => {
  const textArea = document.createElement('textarea')
  textArea.value = text
  document.body.appendChild(textArea)
  textArea.select()
  
  try {
    document.execCommand('copy')
    window.dispatchEvent(new CustomEvent('show-success', {
      detail: '文本已复制到剪贴板'
    }))
  } catch (err) {
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: '复制失败'
    }))
  }
  
  document.body.removeChild(textArea)
}

const exportText = () => {
  const textToExport = isEditing.value ? editingText.value : currentText.value

  if (!textToExport || textToExport.trim() === '') {
    window.dispatchEvent(new CustomEvent('show-warning', {
      detail: '没有可导出的文本内容'
    }))
    return
  }

  // 显示导出格式选择对话框
  showExportDialog.value = true
}

const handleExport = async () => {
  try {
    const textToExport = isEditing.value ? editingText.value : currentText.value

    // 生成默认文件名
    const tabNames = {
      'original': '原始文本',
      'ocr': 'OCR识别',
      'ai': 'AI处理'
    }
    const defaultFileName = `第${props.pageNumber}页_${tabNames[activeTab.value as keyof typeof tabNames]}.${exportFormat.value}`

    if (exportFormat.value === 'docx') {
      // 显示生成提示
      window.dispatchEvent(new CustomEvent('show-info', {
        detail: '正在生成DOCX文档，请稍候...'
      }))

      // 生成DOCX内容
      const docxContent = await generateDocxContent(textToExport, tabNames[activeTab.value as keyof typeof tabNames])

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
      const content = generateExportContent(textToExport)

      const filePath = await SaveFileWithDialog(content, defaultFileName, [
        {
          DisplayName: getFormatDisplayName(exportFormat.value),
          Pattern: `*.${exportFormat.value}`
        }
      ])

      if (!filePath) {
        return
      }

      window.dispatchEvent(new CustomEvent('show-success', {
        detail: `导出成功：${filePath}`
      }))
    }

    showExportDialog.value = false

  } catch (error) {
    console.error('导出失败:', error)
    window.dispatchEvent(new CustomEvent('show-error', {
      detail: `导出失败：${error}`
    }))
  }
}

const generateExportContent = (text: string) => {
  const tabNames = {
    'original': '原始文本',
    'ocr': 'OCR识别',
    'ai': 'AI处理'
  }
  const tabName = tabNames[activeTab.value as keyof typeof tabNames]

  switch (exportFormat.value) {
    case 'markdown':
      return `# 第 ${props.pageNumber} 页 - ${tabName}\n\n${text}\n`
    case 'html':
      return `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>第 ${props.pageNumber} 页 - ${tabName}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; line-height: 1.6; }
        h1 { color: #333; border-bottom: 2px solid #333; padding-bottom: 10px; }
        .content { white-space: pre-wrap; background: #f9f9f9; padding: 20px; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>第 ${props.pageNumber} 页 - ${tabName}</h1>
    <div class="content">${text.replace(/\n/g, '<br>\n')}</div>
</body>
</html>`
    case 'rtf':
      return generateWordContent(text, tabName)
    default: // txt
      return `第 ${props.pageNumber} 页 - ${tabName}\n${'='.repeat(50)}\n\n${text}`
  }
}

const generateWordContent = (text: string, tabName: string) => {
  // 创建RTF格式文档（Rich Text Format）
  // RTF格式兼容性好，可以被Word、LibreOffice等软件打开
  const rtfContent = `{\\rtf1\\ansi\\ansicpg936\\deff0\\deflang2052
{\\fonttbl{\\f0\\fswiss\\fcharset134 Microsoft YaHei;}{\\f1\\fmodern\\fcharset0 Courier New;}}
{\\colortbl;\\red0\\green0\\blue0;\\red0\\green0\\blue255;}
\\viewkind4\\uc1\\pard\\cf1\\lang2052\\f0\\fs28\\b 第 ${props.pageNumber} 页 - ${tabName}\\par
\\par
\\cf0\\fs22\\b0\\f1 ${text.replace(/\\/g, '\\\\').replace(/\{/g, '\\{').replace(/\}/g, '\\}').replace(/\n/g, '\\par\n')}\\par
}`

  return rtfContent
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

// 生成DOCX内容（返回base64字符串用于后端保存）
const generateDocxContent = async (text: string, _tabName: string): Promise<string> => {
  try {
    // 检测文本中是否包含表格
    const hasTable = detectTable(text)

    const doc = new Document({
      sections: [{
        properties: {},
        children: [
          // 直接添加内容，不要标题
          ...(hasTable ? generateTableContent(text) : generateTextContent(text))
        ],
      }],
    })

    // 生成文档 - 使用toBlob而不是toBuffer（浏览器兼容）
    const blob = await Packer.toBlob(doc)

    // 将Blob转换为base64字符串
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
    if (detectTable(line)) {
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
          children: [new TextRun(line)],
          spacing: { after: 200 }
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
              children: [new TextRun(cellText || ' ')] // 防止空字符串
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
    return text.split('\n').map(line =>
      new Paragraph({
        children: [new TextRun(line || ' ')], // 空行用空格代替
        spacing: { after: 200 }
      })
    )
  } catch (error) {
    // 返回一个简单的段落作为后备
    return [new Paragraph({
      children: [new TextRun('文本内容生成失败')]
    })]
  }
}

const close = () => {
  if (hasChanges.value) {
    if (!confirm('有未保存的更改，确定要关闭吗？')) {
      return
    }
  }
  emit('close')
}

// 拖拽相关方法
const startDrag = (event: MouseEvent) => {
  // 防止在拉伸时触发拖拽
  if (isResizing.value) return

  event.preventDefault()
  isDragging.value = true

  const rect = (event.target as HTMLElement).closest('.text-editor-modal')?.getBoundingClientRect()
  if (rect) {
    dragOffset.value = {
      x: event.clientX - rect.left,
      y: event.clientY - rect.top
    }
  }

  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
  event.preventDefault()
}

const onDrag = (event: MouseEvent) => {
  if (!isDragging.value) return

  position.value = {
    x: event.clientX - dragOffset.value.x,
    y: event.clientY - dragOffset.value.y
  }

  // 确保窗口不会拖拽到屏幕外
  position.value.x = Math.max(0, Math.min(position.value.x, window.innerWidth - 400))
  position.value.y = Math.max(0, Math.min(position.value.y, window.innerHeight - 300))
}

const stopDrag = () => {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
}

// 拉伸相关方法
const startResize = (event: MouseEvent, direction: string) => {
  event.preventDefault()
  event.stopPropagation()

  // 防止在拖拽时触发拉伸
  if (isDragging.value) return

  isResizing.value = true
  resizeDirection.value = direction

  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)
}

const onResize = (event: MouseEvent) => {
  if (!isResizing.value) return

  const direction = resizeDirection.value
  const rect = document.querySelector('.text-editor-modal')?.getBoundingClientRect()
  if (!rect) return

  let newWidth = size.value.width
  let newHeight = size.value.height
  let newX = position.value.x
  let newY = position.value.y

  if (direction.includes('right')) {
    newWidth = Math.max(minSize.width, event.clientX - rect.left)
  }
  if (direction.includes('left')) {
    const deltaX = event.clientX - rect.left
    newWidth = Math.max(minSize.width, rect.width - deltaX)
    newX = Math.min(position.value.x + deltaX, position.value.x + rect.width - minSize.width)
  }
  if (direction.includes('bottom')) {
    newHeight = Math.max(minSize.height, event.clientY - rect.top)
  }
  if (direction.includes('top')) {
    const deltaY = event.clientY - rect.top
    newHeight = Math.max(minSize.height, rect.height - deltaY)
    newY = Math.min(position.value.y + deltaY, position.value.y + rect.height - minSize.height)
  }

  size.value = { width: newWidth, height: newHeight }
  position.value = { x: newX, y: newY }
}

const stopResize = () => {
  isResizing.value = false
  resizeDirection.value = ''
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
}
</script>

<template>
  <div class="text-editor">
    <!-- 拉伸手柄 -->
    <div class="resize-handle resize-top" @mousedown="startResize($event, 'top')"></div>
    <div class="resize-handle resize-right" @mousedown="startResize($event, 'right')"></div>
    <div class="resize-handle resize-bottom" @mousedown="startResize($event, 'bottom')"></div>
    <div class="resize-handle resize-left" @mousedown="startResize($event, 'left')"></div>
    <div class="resize-handle resize-top-left" @mousedown="startResize($event, 'top-left')"></div>
    <div class="resize-handle resize-top-right" @mousedown="startResize($event, 'top-right')"></div>
    <div class="resize-handle resize-bottom-left" @mousedown="startResize($event, 'bottom-left')"></div>
    <div class="resize-handle resize-bottom-right" @mousedown="startResize($event, 'bottom-right')"></div>

    <!-- 头部 -->
    <div class="editor-header">
      <div class="header-content">
        <div class="header-icon">📝</div>
        <h3>第 {{ pageNumber }} 页 - 文本编辑</h3>
      </div>
      <button @click="$emit('close')" class="close-btn">×</button>
    </div>

    <!-- 标签页 -->
    <div class="editor-tabs">
      <button 
        :class="['tab-btn', { active: activeTab === 'original' }]"
        @click="activeTab = 'original'"
      >
        原始文本
      </button>
      <button 
        :class="['tab-btn', { active: activeTab === 'ocr' }]"
        @click="activeTab = 'ocr'"
      >
        OCR文本
        <span v-if="activeTab === 'ocr' && canEdit" class="editable-badge">可编辑</span>
      </button>
      <button 
        :class="['tab-btn', { active: activeTab === 'ai' }]"
        @click="activeTab = 'ai'"
      >
        AI处理
        <span v-if="activeTab === 'ai' && canEdit" class="editable-badge">可编辑</span>
      </button>
    </div>

    <!-- 工具栏 -->
    <div class="editor-toolbar">
      <div class="toolbar-left">
        <button v-if="!isEditing && canEdit" @click="startEditing" class="btn btn-primary">
          编辑
        </button>
        <button v-if="isEditing" @click="saveChanges" :disabled="!hasChanges || saving" class="btn btn-success">
          {{ saving ? '保存中...' : '保存' }}
        </button>
        <button v-if="isEditing" @click="cancelEditing" class="btn btn-secondary">
          取消
        </button>
      </div>
      
      <div class="toolbar-right">
        <button @click="copyText" class="btn btn-outline">
          复制
        </button>
        <button @click="exportText" class="btn btn-outline">
          导出
        </button>
      </div>
    </div>

    <!-- 编辑区域 -->
    <div class="editor-content">
      <div v-if="!isEditing" class="text-display">
        <pre class="text-content">{{ currentText }}</pre>
      </div>
      
      <div v-else class="text-edit">
        <textarea
          v-model="editingText"
          class="text-input"
          placeholder="在此编辑文本..."
        ></textarea>
      </div>
    </div>

    <!-- 状态栏 -->
    <div class="editor-status">
      <div class="status-left">
        <span v-if="isEditing">
          字符数: {{ wordCount }} | 行数: {{ lineCount }}
        </span>
        <span v-if="hasChanges" class="changes-indicator">
          * 有未保存的更改
        </span>
      </div>

      <div class="status-right">
        <span class="tab-info">{{ activeTab === 'original' ? '只读' : canEdit ? '可编辑' : '只读' }}</span>
      </div>
    </div>

    <!-- 导出格式选择对话框 -->
    <div v-if="showExportDialog" class="export-dialog-overlay" @click="showExportDialog = false">
      <div class="export-dialog" @click.stop>
        <div class="dialog-header">
          <h4>选择导出格式</h4>
          <button @click="showExportDialog = false" class="close-btn">×</button>
        </div>

        <div class="dialog-content">
          <div class="format-options">
            <label class="format-option">
              <input type="radio" v-model="exportFormat" value="txt" />
              <div class="option-content">
                <div class="option-title">📄 文本文件 (.txt)</div>
                <div class="option-desc">纯文本格式，兼容性最好</div>
              </div>
            </label>

            <label class="format-option">
              <input type="radio" v-model="exportFormat" value="markdown" />
              <div class="option-content">
                <div class="option-title">📝 Markdown (.md)</div>
                <div class="option-desc">支持格式化的轻量级标记语言</div>
              </div>
            </label>

            <label class="format-option">
              <input type="radio" v-model="exportFormat" value="docx" />
              <div class="option-content">
                <div class="option-title">📄 Word文档 (.docx)</div>
                <div class="option-desc">现代Word格式，完美支持表格和复杂格式</div>
              </div>
            </label>
            
            <label class="format-option">
              <input type="radio" v-model="exportFormat" value="html" />
              <div class="option-content">
                <div class="option-title">🌐 HTML (.html)</div>
                <div class="option-desc">网页格式，支持样式和格式</div>
              </div>
            </label>

            <label class="format-option">
              <input type="radio" v-model="exportFormat" value="rtf" />
              <div class="option-content">
                <div class="option-title">📋 RTF文档 (.rtf)</div>
                <div class="option-desc">富文本格式，兼容性好，支持office软件</div>
              </div>
            </label>


          </div>
        </div>

        <div class="dialog-footer">
          <button @click="showExportDialog = false" class="btn btn-secondary">取消</button>
          <button @click="handleExport" class="btn btn-primary">导出</button>
        </div>
      </div>
    </div>

    <!-- 自定义确认对话框 -->
    <div v-if="showConfirmDialog" class="dialog-overlay">
      <div class="dialog-content confirm-dialog">
        <div class="dialog-header">
          <h4>确认操作</h4>
        </div>

        <div class="dialog-body">
          <p>{{ confirmMessage }}</p>
        </div>

        <div class="dialog-footer">
          <button @click="handleCancel" class="btn btn-secondary">取消</button>
          <button @click="handleConfirm" class="btn btn-primary">确定</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>

/* 拉伸手柄 */
.resize-handle {
  position: absolute;
  background: transparent;
  z-index: 1;
  pointer-events: auto;
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

.text-editor {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 12px;
  overflow: hidden;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background: rgba(248, 249, 250, 0.9);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid rgba(224, 224, 224, 0.3);
  cursor: move;
  user-select: none;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.header-icon {
  font-size: 1.2rem;
  opacity: 0.8;
}

.editor-header h3 {
  margin: 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
}

.close-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  font-size: 1.2rem;
  cursor: pointer;
  color: #666;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: all 0.2s ease;
  backdrop-filter: blur(10px);
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: #333;
  transform: scale(1.05);
}

.editor-tabs {
  display: flex;
  background: rgba(248, 249, 250, 0.9);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid rgba(224, 224, 224, 0.3);
}

.tab-btn {
  flex: 1;
  padding: 0.75rem 1rem;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 0.9rem;
  color: #666;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
  position: relative;
  backdrop-filter: blur(10px);
}

.tab-btn:hover {
  background: rgba(233, 236, 239, 0.5);
  color: #333;
}

.tab-btn.active {
  color: #007bff;
  border-bottom-color: #007bff;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
}

.editable-badge {
  display: inline-block;
  background: #28a745;
  color: white;
  font-size: 0.7rem;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  margin-left: 0.5rem;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid rgba(224, 224, 224, 0.3);
}

.toolbar-left,
.toolbar-right {
  display: flex;
  gap: 0.5rem;
}

.editor-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.text-display {
  flex: 1;
  overflow: auto;
  padding: 1.5rem;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.text-display::-webkit-scrollbar {
  width: 8px;
}

.text-display::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.text-display::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.text-display::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.text-content {
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
  color: #333;
}

.text-edit {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 1.5rem;
}

.text-input {
  flex: 1;
  border: 1px solid #ccc;
  border-radius: 4px;
  padding: 1rem;
  font-family: 'Courier New', monospace;
  font-size: 0.9rem;
  line-height: 1.6;
  resize: none;
  outline: none;
  overflow-y: auto;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.text-input::-webkit-scrollbar {
  width: 8px;
}

.text-input::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.text-input::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.text-input::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.text-input:focus {
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.editor-status {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1.5rem;
  background: rgba(248, 249, 250, 0.9);
  backdrop-filter: blur(10px);
  border-top: 1px solid rgba(224, 224, 224, 0.3);
  font-size: 0.8rem;
  color: #666;
}

.changes-indicator {
  color: #ffc107;
  font-weight: 500;
}

.tab-info {
  font-style: italic;
}

.btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s;
}

.btn-primary {
  background: #007bff;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #0056b3;
}

.btn-success {
  background: #28a745;
  color: white;
}

.btn-success:hover:not(:disabled) {
  background: #1e7e34;
}

.btn-secondary {
  background: #6c757d;
  color: white;
}

.btn-secondary:hover {
  background: #545b62;
}

.btn-outline {
  background: transparent;
  color: #6c757d;
  border: 1px solid #6c757d;
}

.btn-outline:hover {
  background: #6c757d;
  color: white;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* 导出对话框样式 */
.export-dialog-overlay {
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

.export-dialog {
  background: white;
  border-radius: 8px;
  width: 90%;
  max-width: 500px;
  max-height: 80vh;
  overflow: hidden;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem;
  border-bottom: 1px solid #e0e0e0;
  background: #f8f9fa;
}

.dialog-header h4 {
  margin: 0;
  color: #333;
  font-size: 1.1rem;
}

.dialog-content {
  padding: 1.5rem;
  max-height: 400px;
  overflow-y: auto;
}

.format-options {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.format-option {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1rem;
  border: 2px solid #e0e0e0;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.format-option:hover {
  border-color: #007bff;
  background: #f8f9ff;
}

.format-option input[type="radio"] {
  margin-top: 0.2rem;
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
}

.option-title {
  font-weight: 500;
  margin-bottom: 0.25rem;
  font-size: 1rem;
}

.option-desc {
  color: #666;
  font-size: 0.9rem;
  line-height: 1.4;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  padding: 1.5rem;
  border-top: 1px solid #e0e0e0;
  background: #f8f9fa;
}

/* 通用对话框样式 */
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
  z-index: 10000;
}

.dialog-content {
  background: rgba(255, 255, 255, 0.98);
  backdrop-filter: blur(12px);
  border-radius: 10px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15), 0 2px 8px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.3);
  overflow: hidden;
  min-width: 300px;
  max-width: 500px;
  max-height: 80vh;
}

.dialog-body {
  padding: 1.5rem;
}

.dialog-body p {
  margin: 0;
  color: #333;
  font-size: 1rem;
  line-height: 1.5;
}

/* 确认对话框特定样式 */
.confirm-dialog {
  min-width: 320px;
  max-width: 380px;
}

.confirm-dialog .dialog-header {
  padding: 1rem 1.25rem 0.5rem;
  border-bottom: none;
  background: transparent;
}

.confirm-dialog .dialog-header h4 {
  font-size: 1rem;
  font-weight: 500;
  color: #333;
  text-align: center;
}

.confirm-dialog .dialog-body {
  text-align: center;
  padding: 0.75rem 1.25rem 1.25rem;
}

.confirm-dialog .dialog-body p {
  font-size: 0.95rem;
  color: #666;
  line-height: 1.4;
  margin: 0;
}

.confirm-dialog .dialog-footer {
  padding: 0.75rem 1.25rem 1.25rem;
  border-top: none;
  background: transparent;
  gap: 0.75rem;
}

.confirm-dialog .btn {
  padding: 0.5rem 1.25rem;
  font-size: 0.9rem;
  border-radius: 6px;
  min-width: 70px;
}
</style>
