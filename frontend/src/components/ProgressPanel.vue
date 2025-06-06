<script lang="ts" setup>
import { computed } from 'vue'

// Props
interface Props {
  progress: {
    total: number
    processed: number
    currentPage: number
    status: string
    error?: string
  }
}

const props = defineProps<Props>()

// 计算属性
const progressPercentage = computed(() => {
  if (props.progress.total === 0) return 0
  return Math.round((props.progress.processed / props.progress.total) * 100)
})

const remainingPages = computed(() => {
  return props.progress.total - props.progress.processed
})

const isComplete = computed(() => {
  return props.progress.processed >= props.progress.total
})

const hasError = computed(() => {
  return !!props.progress.error
})
</script>

<template>
  <div class="progress-overlay">
    <div class="progress-panel">
      <!-- 头部 -->
      <div class="panel-header">
        <h3>{{ hasError ? '处理出错' : isComplete ? '处理完成' : '正在处理' }}</h3>
        <div class="status-indicator" :class="{
          'status-processing': !isComplete && !hasError,
          'status-complete': isComplete,
          'status-error': hasError
        }"></div>
      </div>

      <!-- 进度条 -->
      <div class="progress-section">
        <div class="progress-bar-container">
          <div 
            class="progress-bar" 
            :style="{ width: `${progressPercentage}%` }"
            :class="{
              'progress-error': hasError,
              'progress-complete': isComplete
            }"
          ></div>
        </div>
        <div class="progress-text">
          {{ progressPercentage }}%
        </div>
      </div>

      <!-- 详细信息 -->
      <div class="details-section">
        <div class="detail-row">
          <span class="detail-label">已处理:</span>
          <span class="detail-value">{{ progress.processed }} / {{ progress.total }} 页</span>
        </div>

        <div v-if="!isComplete && !hasError" class="detail-row">
          <span class="detail-label">剩余:</span>
          <span class="detail-value">{{ remainingPages }} 页</span>
        </div>

        <div v-if="progress.currentPage && !isComplete" class="detail-row">
          <span class="detail-label">当前页:</span>
          <span class="detail-value">第 {{ progress.currentPage }} 页</span>
        </div>

        <div class="detail-row">
          <span class="detail-label">状态:</span>
          <span class="detail-value" :class="{
            'status-text-error': hasError,
            'status-text-complete': isComplete
          }">
            {{ hasError ? progress.error : progress.status }}
          </span>
        </div>
      </div>

      <!-- 动画效果 -->
      <div v-if="!isComplete && !hasError" class="processing-animation">
        <div class="dot dot-1"></div>
        <div class="dot dot-2"></div>
        <div class="dot dot-3"></div>
      </div>

      <!-- 完成图标 -->
      <div v-if="isComplete" class="completion-icon">
        <div class="checkmark">✓</div>
      </div>

      <!-- 错误图标 -->
      <div v-if="hasError" class="error-icon">
        <div class="error-mark">✗</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.progress-overlay {
  position: fixed;
  bottom: 2rem;
  right: 2rem;
  z-index: 1000;
}

.progress-panel {
  background: white;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
  padding: 1.5rem;
  min-width: 320px;
  max-width: 400px;
  border: 1px solid #e0e0e0;
  position: relative;
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.panel-header h3 {
  margin: 0;
  color: #333;
  font-size: 1.1rem;
  font-weight: 600;
}

.status-indicator {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  animation: pulse 2s infinite;
}

.status-processing {
  background: #007bff;
}

.status-complete {
  background: #28a745;
  animation: none;
}

.status-error {
  background: #dc3545;
  animation: none;
}

@keyframes pulse {
  0% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.7;
    transform: scale(1.1);
  }
  100% {
    opacity: 1;
    transform: scale(1);
  }
}

.progress-section {
  margin-bottom: 1.5rem;
}

.progress-bar-container {
  width: 100%;
  height: 8px;
  background: #e9ecef;
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 0.5rem;
}

.progress-bar {
  height: 100%;
  background: linear-gradient(90deg, #007bff, #0056b3);
  border-radius: 4px;
  transition: width 0.3s ease;
  position: relative;
}

.progress-bar::after {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(255, 255, 255, 0.3),
    transparent
  );
  animation: shimmer 2s infinite;
}

.progress-complete {
  background: linear-gradient(90deg, #28a745, #1e7e34);
}

.progress-error {
  background: linear-gradient(90deg, #dc3545, #c82333);
}

@keyframes shimmer {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(100%);
  }
}

.progress-text {
  text-align: center;
  font-weight: 600;
  color: #333;
  font-size: 1.1rem;
}

.details-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.9rem;
}

.detail-label {
  color: #666;
  font-weight: 500;
}

.detail-value {
  color: #333;
  font-weight: 600;
}

.status-text-error {
  color: #dc3545;
}

.status-text-complete {
  color: #28a745;
}

.processing-animation {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 0.5rem;
  margin-top: 1rem;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #007bff;
  animation: bounce 1.4s infinite ease-in-out;
}

.dot-1 {
  animation-delay: -0.32s;
}

.dot-2 {
  animation-delay: -0.16s;
}

.dot-3 {
  animation-delay: 0s;
}

@keyframes bounce {
  0%, 80%, 100% {
    transform: scale(0.8);
    opacity: 0.5;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}

.completion-icon,
.error-icon {
  display: flex;
  justify-content: center;
  align-items: center;
  margin-top: 1rem;
}

.checkmark {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: #28a745;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.5rem;
  font-weight: bold;
  animation: checkmarkAppear 0.5s ease-out;
}

.error-mark {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: #dc3545;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.5rem;
  font-weight: bold;
  animation: errorAppear 0.5s ease-out;
}

@keyframes checkmarkAppear {
  0% {
    transform: scale(0);
    opacity: 0;
  }
  50% {
    transform: scale(1.2);
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes errorAppear {
  0% {
    transform: scale(0);
    opacity: 0;
  }
  50% {
    transform: scale(1.2);
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}

/* 响应式设计 */
@media (max-width: 480px) {
  .progress-overlay {
    bottom: 1rem;
    right: 1rem;
    left: 1rem;
  }

  .progress-panel {
    min-width: auto;
    max-width: none;
  }
}
</style>
