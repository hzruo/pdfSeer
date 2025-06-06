<script setup lang="ts">
import { ref, computed } from 'vue'

interface DialogProps {
  show: boolean
  title?: string
  message: string
  type?: 'info' | 'success' | 'warning' | 'error' | 'confirm'
  confirmText?: string
  cancelText?: string
  showCancel?: boolean
}

const props = withDefaults(defineProps<DialogProps>(), {
  title: '',
  type: 'info',
  confirmText: '确定',
  cancelText: '取消',
  showCancel: false
})

const emit = defineEmits<{
  confirm: []
  cancel: []
  close: []
}>()

// 计算图标
const dialogIcon = computed(() => {
  switch (props.type) {
    case 'success':
      return '✅'
    case 'warning':
      return '⚠️'
    case 'error':
      return '❌'
    case 'confirm':
      return '❓'
    default:
      return 'ℹ️'
  }
})

// 计算样式类
const dialogClass = computed(() => {
  return `dialog-${props.type}`
})

const handleConfirm = () => {
  emit('confirm')
  emit('close')
}

const handleCancel = () => {
  emit('cancel')
  emit('close')
}

const handleOverlayClick = (event: MouseEvent) => {
  if (event.target === event.currentTarget) {
    if (props.showCancel) {
      handleCancel()
    } else {
      handleConfirm()
    }
  }
}
</script>

<template>
  <div v-if="show" class="dialog-overlay" @click="handleOverlayClick">
    <div class="dialog-container" :class="dialogClass">
      <!-- 头部 -->
      <div class="dialog-header">
        <div class="dialog-icon">{{ dialogIcon }}</div>
        <h3 v-if="title" class="dialog-title">{{ title }}</h3>
      </div>

      <!-- 内容 -->
      <div class="dialog-content">
        <p class="dialog-message">{{ message }}</p>
      </div>

      <!-- 底部按钮 -->
      <div class="dialog-footer">
        <button 
          v-if="showCancel" 
          @click="handleCancel" 
          class="dialog-btn dialog-btn-cancel"
        >
          {{ cancelText }}
        </button>
        <button 
          @click="handleConfirm" 
          class="dialog-btn dialog-btn-confirm"
          :class="`dialog-btn-${type}`"
        >
          {{ confirmText }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  backdrop-filter: blur(4px);
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.dialog-container {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  width: 90%;
  max-width: 400px;
  min-width: 300px;
  overflow: hidden;
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

.dialog-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1.5rem 1.5rem 1rem 1.5rem;
}

.dialog-icon {
  font-size: 1.5rem;
  flex-shrink: 0;
}

.dialog-title {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: #333;
}

.dialog-content {
  padding: 0 1.5rem 1.5rem 1.5rem;
}

.dialog-message {
  margin: 0;
  font-size: 0.95rem;
  line-height: 1.5;
  color: #555;
}

.dialog-footer {
  display: flex;
  gap: 0.75rem;
  padding: 1rem 1.5rem 1.5rem 1.5rem;
  justify-content: flex-end;
}

.dialog-btn {
  padding: 0.6rem 1.2rem;
  border: none;
  border-radius: 8px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 80px;
}

.dialog-btn-cancel {
  background: #f8f9fa;
  color: #6c757d;
  border: 1px solid #dee2e6;
}

.dialog-btn-cancel:hover {
  background: #e9ecef;
  color: #495057;
}

.dialog-btn-confirm {
  color: white;
}

.dialog-btn-info {
  background: #007bff;
}

.dialog-btn-info:hover {
  background: #0056b3;
}

.dialog-btn-success {
  background: #28a745;
}

.dialog-btn-success:hover {
  background: #1e7e34;
}

.dialog-btn-warning {
  background: #ffc107;
  color: #212529;
}

.dialog-btn-warning:hover {
  background: #e0a800;
}

.dialog-btn-error {
  background: #dc3545;
}

.dialog-btn-error:hover {
  background: #c82333;
}

.dialog-btn-confirm {
  background: #007bff;
}

.dialog-btn-confirm:hover {
  background: #0056b3;
}

/* 对话框类型样式 */
.dialog-success .dialog-header {
  background: linear-gradient(135deg, rgba(40, 167, 69, 0.1) 0%, rgba(40, 167, 69, 0.05) 100%);
}

.dialog-warning .dialog-header {
  background: linear-gradient(135deg, rgba(255, 193, 7, 0.1) 0%, rgba(255, 193, 7, 0.05) 100%);
}

.dialog-error .dialog-header {
  background: linear-gradient(135deg, rgba(220, 53, 69, 0.1) 0%, rgba(220, 53, 69, 0.05) 100%);
}

.dialog-confirm .dialog-header {
  background: linear-gradient(135deg, rgba(0, 123, 255, 0.1) 0%, rgba(0, 123, 255, 0.05) 100%);
}

.dialog-info .dialog-header {
  background: linear-gradient(135deg, rgba(23, 162, 184, 0.1) 0%, rgba(23, 162, 184, 0.05) 100%);
}
</style>
