<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

// 错误类型
interface ErrorInfo {
  id: string
  type: 'error' | 'warning' | 'info' | 'success'
  title: string
  message: string
  timestamp: Date
  duration?: number
}

// 响应式数据
const errors = ref<ErrorInfo[]>([])
const maxErrors = 5

// 生命周期
onMounted(() => {
  // 监听 Wails 后端事件
  EventsOn('error', handleError)
  EventsOn('processing-error', handleProcessingError)
  EventsOn('ai-processing-error', handleAIError)

  // 监听 DOM 事件（前端发送的事件）
  window.addEventListener('warning', handleWarningEvent)
  window.addEventListener('info', handleInfoEvent)
  window.addEventListener('success', handleSuccessEvent)
  window.addEventListener('error', handleErrorEvent)
})

onUnmounted(() => {
  // 清理 Wails 事件监听
  EventsOff('error')
  EventsOff('processing-error')
  EventsOff('ai-processing-error')

  // 清理 DOM 事件监听
  window.removeEventListener('warning', handleWarningEvent)
  window.removeEventListener('info', handleInfoEvent)
  window.removeEventListener('success', handleSuccessEvent)
  window.removeEventListener('error', handleErrorEvent)
})

// 方法
const generateId = () => {
  return Date.now().toString(36) + Math.random().toString(36).substr(2)
}

const addError = (errorInfo: Omit<ErrorInfo, 'id' | 'timestamp'>) => {
  const error: ErrorInfo = {
    ...errorInfo,
    id: generateId(),
    timestamp: new Date(),
  }

  errors.value.unshift(error)

  // 限制错误数量
  if (errors.value.length > maxErrors) {
    errors.value = errors.value.slice(0, maxErrors)
  }

  // 自动移除
  if (error.duration && error.duration > 0) {
    setTimeout(() => {
      removeError(error.id)
    }, error.duration)
  }
}

const removeError = (id: string) => {
  const index = errors.value.findIndex(error => error.id === id)
  if (index > -1) {
    errors.value.splice(index, 1)
  }
}

const clearAll = () => {
  errors.value = []
}

const handleError = (message: string) => {
  addError({
    type: 'error',
    title: '错误',
    message: message,
    duration: 8000,
  })
}

const handleProcessingError = (message: string) => {
  addError({
    type: 'error',
    title: '处理错误',
    message: message,
    duration: 10000,
  })
}

const handleAIError = (message: string) => {
  addError({
    type: 'error',
    title: 'AI处理错误',
    message: message,
    duration: 10000,
  })
}

const handleWarning = (message: string) => {
  addError({
    type: 'warning',
    title: '警告',
    message: message,
    duration: 6000,
  })
}

const handleInfo = (message: string) => {
  addError({
    type: 'info',
    title: '信息',
    message: message,
    duration: 4000,
  })
}

const handleSuccess = (message: string) => {
  addError({
    type: 'success',
    title: '成功',
    message: message,
    duration: 3000,
  })
}

// DOM 事件处理器
const handleWarningEvent = (event: any) => {
  handleWarning(event.detail)
}

const handleInfoEvent = (event: any) => {
  handleInfo(event.detail)
}

const handleSuccessEvent = (event: any) => {
  handleSuccess(event.detail)
}

const handleErrorEvent = (event: any) => {
  handleError(event.detail)
}

const getErrorIcon = (type: string) => {
  switch (type) {
    case 'error': return '❌'
    case 'warning': return '⚠️'
    case 'info': return 'ℹ️'
    case 'success': return '✅'
    default: return '📝'
  }
}

const getErrorClass = (type: string) => {
  switch (type) {
    case 'error': return 'error-item-error'
    case 'warning': return 'error-item-warning'
    case 'info': return 'error-item-info'
    case 'success': return 'error-item-success'
    default: return 'error-item-info'
  }
}

const formatTime = (date: Date) => {
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}
</script>

<template>
  <div class="error-handler">
    <div v-if="errors.length > 0" class="errors-container">
      <div class="errors-header">
        <h4>消息通知</h4>
        <button @click="clearAll" class="clear-btn">清空</button>
      </div>
      
      <div class="errors-list">
        <div 
          v-for="error in errors" 
          :key="error.id"
          :class="['error-item', getErrorClass(error.type)]"
        >
          <div class="error-icon">
            {{ getErrorIcon(error.type) }}
          </div>
          
          <div class="error-content">
            <div class="error-title">{{ error.title }}</div>
            <div class="error-message">{{ error.message }}</div>
            <div class="error-time">{{ formatTime(error.timestamp) }}</div>
          </div>
          
          <button @click="removeError(error.id)" class="error-close">
            ×
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.error-handler {
  position: fixed;
  top: 1rem;
  right: 1rem;
  z-index: 2000;
  max-width: 400px;
}

.errors-container {
  background: white;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
  border: 1px solid #e0e0e0;
  overflow: hidden;
}

.errors-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  background: #f8f9fa;
  border-bottom: 1px solid #e0e0e0;
}

.errors-header h4 {
  margin: 0;
  font-size: 0.9rem;
  color: #333;
}

.clear-btn {
  background: none;
  border: none;
  color: #666;
  cursor: pointer;
  font-size: 0.8rem;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
}

.clear-btn:hover {
  background: #e9ecef;
}

.errors-list {
  max-height: 400px;
  overflow-y: auto;
}

.error-item {
  display: flex;
  align-items: flex-start;
  padding: 1rem;
  border-bottom: 1px solid #f0f0f0;
  transition: background-color 0.2s;
}

.error-item:hover {
  background: #f8f9fa;
}

.error-item:last-child {
  border-bottom: none;
}

.error-item-error {
  border-left: 4px solid #dc3545;
}

.error-item-warning {
  border-left: 4px solid #ffc107;
}

.error-item-info {
  border-left: 4px solid #17a2b8;
}

.error-item-success {
  border-left: 4px solid #28a745;
}

.error-icon {
  font-size: 1.2rem;
  margin-right: 0.75rem;
  margin-top: 0.1rem;
}

.error-content {
  flex: 1;
  min-width: 0;
}

.error-title {
  font-weight: 600;
  color: #333;
  font-size: 0.9rem;
  margin-bottom: 0.25rem;
}

.error-message {
  color: #666;
  font-size: 0.85rem;
  line-height: 1.4;
  margin-bottom: 0.25rem;
  word-wrap: break-word;
}

.error-time {
  color: #999;
  font-size: 0.75rem;
}

.error-close {
  background: none;
  border: none;
  color: #999;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  margin-left: 0.5rem;
}

.error-close:hover {
  background: #f0f0f0;
  color: #666;
}

/* 动画效果 */
.error-item {
  animation: slideIn 0.3s ease-out;
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

/* 响应式设计 */
@media (max-width: 480px) {
  .error-handler {
    top: 0.5rem;
    right: 0.5rem;
    left: 0.5rem;
    max-width: none;
  }
  
  .error-item {
    padding: 0.75rem;
  }
  
  .error-message {
    font-size: 0.8rem;
  }
}
</style>
