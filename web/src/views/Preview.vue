<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowDown, Refresh } from '@element-plus/icons-vue'
import { generateApi } from '../api'
import { desktopOpenInBrowser } from '../utils/desktop'
import TaskProgressDialog from '../components/TaskProgressDialog.vue'
import type { Task } from '../types'

const iframeRef = ref<HTMLIFrameElement | null>(null)
const src = ref('/preview/')
const dialogVisible = ref(false)
const taskId = ref(0)

function refresh() {
  // 追加时间戳强制绕过缓存，重新拉取最新生成产物
  src.value = `/preview/?_t=${Date.now()}`
}

function openExternal() {
  const path = '/preview/'
  // 桌面端 webview 没有新窗口能力，交给系统浏览器打开
  if (desktopOpenInBrowser(path)) return
  window.open(path, '_blank')
}

async function regenerate(mode: 'full' | 'increment') {
  try {
    const res = mode === 'full' ? await generateApi.full() : await generateApi.increment()
    taskId.value = res.task_id
    dialogVisible.value = true
  } catch {
    ElMessage.error('生成任务启动失败')
  }
}

function onTaskDone(task: Task) {
  if (task.status === 'success') {
    ElMessage.success('生成完成，正在刷新预览')
    refresh()
  } else if (task.status === 'failed') {
    ElMessage.error(task.error || '生成失败')
  }
}
</script>

<template>
  <div class="preview-page">
    <div class="preview-toolbar">
      <div class="toolbar-left">
        <el-button :icon="Refresh" @click="refresh">刷新预览</el-button>
        <el-button @click="openExternal">新窗口打开</el-button>
      </div>
      <div class="toolbar-right">
        <el-dropdown @command="(c: string) => regenerate(c as 'full' | 'increment')">
          <el-button type="primary">
            重新生成<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="increment">增量生成</el-dropdown-item>
              <el-dropdown-item command="full">全量生成</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>
    <iframe
      ref="iframeRef"
      :src="src"
      class="preview-frame"
      frameborder="0"
      title="站点预览"
    />
    <TaskProgressDialog v-model="dialogVisible" :task-id="taskId" @done="onTaskDone" />
  </div>
</template>

<style scoped>
.preview-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 56px);
  background: #fff;
}
.preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid #eef0f4;
  background: #fafbfc;
  gap: 12px;
}
.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.preview-frame {
  flex: 1 1 auto;
  width: 100%;
  border: none;
  background: #fff;
}
</style>
