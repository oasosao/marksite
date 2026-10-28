<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { generateApi } from '../api'
import { taskTypeText } from '../utils/format'
import type { Task } from '../types'

const props = defineProps<{ modelValue: boolean; taskId: number }>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'done', task: Task): void
}>()

const task = ref<Task | null>(null)
const logBox = ref<HTMLElement | null>(null)
let timer: number | undefined

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})

const running = computed(() => task.value?.status === 'running' || task.value?.status === 'pending')

const statusText: Record<string, string> = {
  pending: '排队中',
  running: '运行中',
  success: '已完成',
  failed: '失败',
  canceled: '已取消',
}

const logLines = computed(() => (task.value?.log || '').split('\n').filter(Boolean).slice(-200))

async function poll() {
  if (!props.taskId) return
  try {
    task.value = await generateApi.task(props.taskId)
    requestAnimationFrame(() => {
      if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
    })
    if (!running.value) {
      stop()
      emit('done', task.value)
    }
  } catch {
    stop()
  }
}

function start() {
  stop()
  task.value = null
  poll()
  timer = window.setInterval(poll, 1000)
}

function stop() {
  if (timer) {
    window.clearInterval(timer)
    timer = undefined
  }
}

async function cancel() {
  if (!task.value) return
  await generateApi.cancel(task.value.id)
  ElMessage.success('已请求取消任务')
}

watch(
  () => [props.modelValue, props.taskId],
  ([open]) => {
    if (open) start()
    else stop()
  },
  { immediate: true },
)

onBeforeUnmount(stop)
</script>

<template>
  <el-dialog v-model="visible" title="任务进度" width="600px" :close-on-click-modal="false">
    <div v-if="task">
      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px">
        <span>任务 #{{ task.id }} · {{ taskTypeText(task.type) }}</span>
        <el-tag :type="task.status === 'success' ? 'success' : task.status === 'failed' ? 'danger' : task.status === 'canceled' ? 'info' : 'primary'">
          {{ statusText[task.status] || task.status }}
        </el-tag>
      </div>
      <el-progress :percentage="task.progress || 0" :status="task.status === 'failed' ? 'exception' : undefined" />
      <div class="muted" style="margin: 6px 0 10px">
        {{ task.done }} / {{ task.total }} 项 <span v-if="task.error">· {{ task.error }}</span>
      </div>
      <div ref="logBox" class="task-log mono">
        <div v-for="(line, idx) in logLines" :key="idx">{{ line }}</div>
        <div v-if="!logLines.length" class="muted">暂无日志</div>
      </div>
    </div>
    <div v-else class="muted">加载中…</div>
    <template #footer>
      <el-button v-if="running" type="danger" plain @click="cancel">取消任务</el-button>
      <el-button @click="visible = false">后台继续</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.task-log {
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  padding: 10px 12px;
  height: 240px;
  overflow: auto;
  line-height: 1.6;
  white-space: pre-wrap;
}
</style>
