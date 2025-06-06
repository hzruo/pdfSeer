<script lang="ts" setup>
import { ref, computed, watch, nextTick } from 'vue'
import { SelectFile, GetPageImage, GetPDFPath } from '../../wailsjs/go/main/App'

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

// 图片模态对话框状态
const showImageModal = ref(false)

// 计算属性
const hasDocument = computed(() => props.document && props.document.pages)
const totalPages = computed(() => props.document?.page_count || 0)
const currentPageData = computed(() => {
  if (!hasDocument.value || currentPage.value < 1) return null
  return props.document.pages[currentPage.value - 1]
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
  isDragging.value = true
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
  event.preventDefault()
}

const onDrag = (event: MouseEvent) => {
  if (!isDragging.value) return

  const container = document.querySelector('.single-view') as HTMLElement
  if (!container) return

  const rect = container.getBoundingClientRect()
  const newPosition = ((event.clientX - rect.left) / rect.width) * 100
  splitPosition.value = Math.max(20, Math.min(80, newPosition))
}

const stopDrag = () => {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
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
                @click="activeTab = 'original'"
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
                </div>
                <div class="result-text">
                  {{ currentPageData.ai_text || '暂无 AI 处理结果' }}
                </div>
              </div>

              <div v-if="activeTab === 'original'" class="result-panel">
                <div class="result-header">
                  <h5>PDF 原生文本</h5>
                </div>
                <div class="result-text">
                  {{ currentPageData.text || '暂无原生文本' }}
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
  </div>
</template>

<style scoped>
.pdf-viewer {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.file-drop-zone {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.8);
  border: 2px dashed rgba(102, 126, 234, 0.3);
  margin: 2rem;
  border-radius: 16px;
  backdrop-filter: blur(10px);
  transition: all 0.3s ease;
}

.file-drop-zone:hover {
  border-color: rgba(102, 126, 234, 0.6);
  background: rgba(255, 255, 255, 0.9);
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.15);
}

.drop-content {
  text-align: center;
  padding: 3rem 2rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.drop-icon {
  font-size: 4rem;
  margin-bottom: 1.5rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.drop-content h3 {
  margin: 0 0 1rem 0;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 1.5rem;
  font-weight: 600;
}

.drop-content p {
  margin: 0 0 2rem 0;
  color: #666;
  font-size: 1.1rem;
  line-height: 1.6;
}

.viewer-content {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.viewer-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.document-info {
  font-size: 0.9rem;
  color: #666;
  max-width: 250px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: rgba(102, 126, 234, 0.1);
  padding: 0.5rem 1rem;
  border-radius: 20px;
  font-weight: 500;
}

.toolbar-center {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.page-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.page-input {
  width: 60px;
  padding: 0.25rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  text-align: center;
}

.single-view {
  flex: 1;
  overflow: auto;
  padding: 1rem;
}

.page-container {
  max-width: 1000px;
  margin: 0 auto;
}

.page-wrapper {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.page-selector {
  padding: 1rem 1.5rem;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.05) 0%, rgba(118, 75, 162, 0.05) 100%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.page-image-container {
  display: flex;
  justify-content: center;
  padding: 1rem;
  min-height: 400px;
}

.page-image {
  max-width: 100%;
  max-height: 800px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  border-radius: 4px;
}

.loading-placeholder,
.error-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 400px;
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

/* 页面结果面板样式 */
.page-results-panel {
  background: #f8f9fa;
  border-top: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

/* 右侧结果面板样式 */
.results-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: #f8f9fa;
}

.results-panel .page-info-section {
  flex: 0 0 auto;
}

.results-panel .parsing-results {
  flex: 1;
  min-height: 0;
}

.page-info-section {
  padding: 1rem;
  border-bottom: 1px solid #e0e0e0;
}

.page-info-section h4 {
  margin: 0 0 1rem 0;
  color: #333;
  font-size: 1.1rem;
}

.page-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}

.meta-item {
  font-size: 0.9rem;
  color: #666;
}

.status-processed {
  color: #28a745;
  font-weight: 500;
}

.status-unprocessed {
  color: #dc3545;
  font-weight: 500;
}

/* 解析结果样式 */
.parsing-results {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: white;
  min-height: 300px;
  max-height: 75vh; /* 设置最大高度为视口高度的75% */
  overflow-y: scroll; /* 强制显示垂直滚动条 */
  border: 1px solid #e0e0e0; /* 添加边框便于调试 */
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.parsing-results::-webkit-scrollbar {
  width: 8px;
}

.parsing-results::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.parsing-results::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.parsing-results::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.results-tabs {
  flex: 0 0 auto;
  display: flex;
  border-bottom: 1px solid #e0e0e0;
}

.tab-btn {
  padding: 0.75rem 1rem;
  border: none;
  background: transparent;
  color: #666;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
  font-size: 0.9rem;
}

.tab-btn:hover {
  background: #f8f9fa;
  color: #333;
}

.tab-btn.active {
  color: #007bff;
  border-bottom-color: #007bff;
  background: white;
}

.results-content {
  flex: 1;
  overflow: auto;
  min-height: 0;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.results-content::-webkit-scrollbar {
  width: 8px;
}

.results-content::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.results-content::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.results-content::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.result-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding: 1rem;
}

.result-header {
  flex: 0 0 auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.result-header h5 {
  margin: 0;
  color: #333;
  font-size: 1rem;
}

.edit-btn {
  background: #007bff;
  color: white;
  border: none;
}

.edit-btn:hover {
  background: #0056b3;
}

.result-text {
  flex: 1;
  overflow-y: auto;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  font-size: 0.9rem;
  line-height: 1.6;
  white-space: pre-wrap;
  color: #333;
  min-height: 0;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.result-text::-webkit-scrollbar {
  width: 8px;
}

.result-text::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.result-text::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.result-text::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.grid-view {
  flex: 1;
  overflow: auto;
  padding: 1rem;
  background: #f8f9fa;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.grid-view::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.grid-view::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.grid-view::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.grid-view::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.pages-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(var(--grid-size, 200px), 1fr));
  gap: 1rem;
  max-width: 100%;
}

.grid-page {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.grid-page:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 8px rgba(0,0,0,0.15);
}

.grid-page.selected {
  border: 2px solid #007bff;
}

.grid-page.current {
  border: 2px solid #28a745;
}

.grid-page-header {
  padding: 0.5rem;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.9rem;
}

.grid-page-image {
  height: calc(var(--grid-size, 200px) * 1.25);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #fafafa;
}

.grid-page-image img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.grid-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8f9fa;
  color: #666;
  font-size: 0.9rem;
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

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.4);
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

.btn-nav {
  background: rgba(255, 255, 255, 0.9);
  color: #333;
  border: 1px solid rgba(0, 0, 0, 0.1);
  padding: 0.5rem 1rem;
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.btn-nav:hover:not(:disabled) {
  background: rgba(255, 255, 255, 1);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.btn-nav:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.btn-large {
  padding: 1rem 2rem;
  font-size: 1.1rem;
  border-radius: 12px;
}

.btn-small {
  padding: 0.5rem 1rem;
  font-size: 0.8rem;
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.page-checkbox {
  margin: 0;
}

/* 浏览器预览样式 */
.browser-view {
  flex: 1;
  display: flex;
  gap: 1rem;
  padding: 1rem;
}

.browser-container {
  flex: 1;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  overflow: hidden;
}

.pdf-iframe {
  width: 100%;
  height: 100%;
  border: none;
  min-height: 600px;
}

.browser-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 600px;
  color: #666;
}

.page-selection-panel {
  width: 250px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  padding: 1rem;
  max-height: 600px;
  overflow-y: auto;
}

.page-selection-panel h4 {
  margin: 0 0 1rem 0;
  color: #333;
  font-size: 1rem;
}

.page-grid {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.page-checkbox-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.page-checkbox-item:hover {
  background: #f8f9fa;
}

.page-checkbox-item input[type="checkbox"] {
  margin: 0;
}

.selection-summary {
  padding: 0.5rem;
  background: #f8f9fa;
  border-radius: 4px;
  font-size: 0.9rem;
  color: #666;
  text-align: center;
}

.browser-info {
  color: #666;
  font-size: 0.9rem;
}

.current-mode {
  font-size: 0.8rem;
  color: #666;
  margin-left: 0.5rem;
}

/* 网格控制样式 */
.grid-controls {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}

.grid-info {
  color: #666;
  font-size: 0.9rem;
}

.grid-size-control {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.9rem;
}

.grid-size-control label {
  color: #666;
  margin: 0;
}

.size-slider {
  width: 100px;
}

.size-value {
  color: #333;
  font-weight: 500;
  min-width: 50px;
}

.placeholder-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.placeholder-text {
  font-size: 0.8rem;
  color: #999;
}

/* 单页视图左右分栏布局 */
.single-view {
  flex: 1;
  display: flex;
  flex-direction: row;
  overflow: hidden;
  padding: 0;
  gap: 0;
}

/* 左侧预览面板 */
.preview-panel {
  display: flex;
  flex-direction: column;
  background: white;
  border-right: 1px solid #e0e0e0;
  min-width: 300px;
  overflow: hidden;
}

.preview-header {
  flex: 0 0 auto;
  padding: 1rem;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
}

.page-selector {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.zoom-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.zoom-level {
  min-width: 50px;
  text-align: center;
  font-size: 0.9rem;
  color: #666;
}

.image-preview-container {
  flex: 1;
  overflow: auto;
  padding: 1rem;
  background: #fafafa;
  /* 自定义滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: #ccc #f0f0f0;
}

.image-preview-container::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.image-preview-container::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

.image-preview-container::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

.image-preview-container::-webkit-scrollbar-thumb:hover {
  background: #999;
}

.image-wrapper {
  display: flex;
  justify-content: center;
  align-items: flex-start; /* 改为顶部对齐，避免图片被裁剪 */
  min-height: 100%;
  transition: transform 0.2s ease;
  transform-origin: center;
}

.preview-image {
  max-width: 100%;
  height: auto;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  border-radius: 4px;
  display: block;
  cursor: pointer; /* 添加点击光标 */
  /* 确保图片可以完整显示 */
  object-fit: contain;
  transition: transform 0.2s ease;
}

.preview-image:hover {
  transform: scale(1.02);
}



/* 图片模态对话框样式 */
.image-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999; /* 提高层级，确保在最上层 */
  backdrop-filter: blur(4px);
}

.image-modal {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  width: 70vw; /* 减少宽度，减少左右空白 */
  height: 85vh; /* 稍微减少高度，避免被标题栏遮挡 */
  max-width: 900px; /* 减少最大宽度 */
  max-height: 700px; /* 减少最大高度 */
  min-width: 600px; /* 设置最小宽度，确保在小屏幕下可用 */
  min-height: 500px; /* 设置最小高度 */
  display: flex;
  flex-direction: column;
  overflow: hidden;
  margin-top: 2vh; /* 添加顶部边距，避免被标题栏遮挡 */
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
}

.modal-header h3 {
  margin: 0;
  color: #333;
  font-size: 1.2rem;
}

.modal-controls {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.modal-tip {
  font-size: 0.9rem;
  color: #666;
  font-style: italic;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: #666;
  cursor: pointer;
  padding: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s;
}

.close-btn:hover {
  background: #e0e0e0;
  color: #333;
}

.modal-body {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  overflow: auto;
  background: #fafafa;
}

.image-container {
  max-width: 100%;
  max-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
  cursor: grab;
}

.modal-image:active {
  cursor: grabbing;
}

.modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  background: #f8f9fa;
  border-top: 1px solid #e0e0e0;
}

.image-info {
  color: #666;
  font-size: 0.9rem;
}

.modal-actions {
  display: flex;
  gap: 0.5rem;
}

/* 响应式设计 - 小屏幕优化 */
@media (max-width: 768px) {
  .image-modal {
    width: 95vw;
    height: 80vh;
    min-width: 320px;
    min-height: 400px;
    margin-top: 5vh; /* 小屏幕下增加顶部边距 */
  }

  .modal-header {
    padding: 0.8rem 1rem;
  }

  .modal-header h3 {
    font-size: 1rem;
  }

  .modal-tip {
    display: none; /* 小屏幕下隐藏提示文字 */
  }

  .modal-actions {
    flex-wrap: wrap;
    gap: 0.3rem;
  }

  .btn {
    font-size: 0.8rem;
    padding: 6px 12px;
  }
}

@media (max-height: 600px) {
  .image-modal {
    height: 90vh;
    margin-top: 1vh; /* 低高度屏幕下减少顶部边距 */
  }

  .modal-header {
    padding: 0.5rem 1rem;
  }

  .modal-footer {
    padding: 0.5rem 1rem;
  }
}

/* 分割线 */
.split-divider {
  width: 4px;
  background: #e0e0e0;
  cursor: col-resize;
  position: relative;
  transition: background-color 0.2s;
}

.split-divider:hover,
.split-divider.dragging {
  background: #007bff;
}

.split-divider::before {
  content: '';
  position: absolute;
  left: -2px;
  right: -2px;
  top: 0;
  bottom: 0;
}

/* 右侧结果面板 */
.results-panel {
  display: flex;
  flex-direction: column;
  background: white;
  min-width: 300px;
  overflow: hidden;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #e0e0e0;
}

.header-buttons {
  display: flex;
  gap: 0.5rem;
}

.btn-warning {
  background: #ffc107;
  color: #212529;
  border: 1px solid #ffc107;
}

.btn-warning:hover {
  background: #e0a800;
  border-color: #d39e00;
}

.size-pending {
  color: #6c757d;
  font-style: italic;
  font-size: 0.9em;
}
</style>
