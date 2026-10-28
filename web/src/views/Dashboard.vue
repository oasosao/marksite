<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { backupApi, ftsApi, generateApi, statsApi } from '../api'
import type { Stats, Task } from '../types'
import { formatDate, formatSize, taskTypeText } from '../utils/format'
import TaskProgressDialog from '../components/TaskProgressDialog.vue'

const router = useRouter()
const stats = ref<Stats | null>(null)
const loading = ref(false)
const dialogVisible = ref(false)
const taskId = ref(0)

const statusText: Record<string, string> = {
  pending: '排队中', running: '运行中', success: '成功', failed: '失败', canceled: '已取消',
}

async function load() {
  loading.value = true
  try {
    stats.value = await statsApi.get()
  } finally {
    loading.value = false
  }
}

async function runGenerate(mode: 'full' | 'increment') {
  if (mode === 'full') {
    await ElMessageBox.confirm(
      '全量生成会清空 output 目录并重建全部静态文件，确认继续？',
      '全量生成确认',
      { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' },
    )
  }
  const res = mode === 'full' ? await generateApi.full() : await generateApi.increment()
  taskId.value = res.task_id
  dialogVisible.value = true
}

async function runRebuild() {
  const res = await ftsApi.rebuild()
  taskId.value = res.task_id
  dialogVisible.value = true
}

async function createBackup() {
  const res = await backupApi.create()
  ElMessage.success(`备份已创建：${res.name}`)
  load()
}

function onTaskDone(task: Task) {
  load()
  if (task.status === 'success') ElMessage.success('任务已完成')
  else if (task.status === 'failed') ElMessage.error(task.error || '任务失败')
}

onMounted(load)
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="page-header">
      <div>
        <h2 class="page-title">仪表盘</h2>
        <p class="page-subtitle">内容、资源与索引概览</p>
      </div>
      <el-button @click="load">刷新</el-button>
    </div>

    <div class="stat-grid" v-if="stats">
      <div class="stat-card">
        <div class="stat-label">文章</div>
        <div class="stat-value">{{ stats.posts.total }}</div>
        <div class="stat-extra">已发布 {{ stats.posts.published }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">页面</div>
        <div class="stat-value">{{ stats.pages.total }}</div>
        <div class="stat-extra">已发布 {{ stats.pages.published }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">资源</div>
        <div class="stat-value">{{ stats.assets.count }}</div>
        <div class="stat-extra">占用 {{ formatSize(stats.assets.size) }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">索引文档</div>
        <div class="stat-value">{{ stats.fts.docs }}</div>
        <div class="stat-extra">{{ stats.fts.exists ? '索引已建立' : '索引不存在' }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">分类</div>
        <div class="stat-value">{{ stats.dirs }}</div>
        <div class="stat-extra">数据库 {{ formatSize(stats.db_size) }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">标签</div>
        <div class="stat-value">{{ stats.tags }}</div>
        <div class="stat-extra">生成到 /tags/{slug}.html</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">备份</div>
        <div class="stat-value">{{ stats.backups.count }}</div>
        <div class="stat-extra">最近生成：{{ stats.last_generate ? formatDate(stats.last_generate.finished_at || stats.last_generate.created_at) : '暂无' }}</div>
      </div>
    </div>

    <div class="card" style="margin-bottom: 18px">
      <h3 style="margin: 0 0 12px; font-size: 15px">快捷操作</h3>
      <div class="page-actions">
        <el-button type="primary" @click="router.push('/posts/edit/0')">新建文章</el-button>
        <el-button @click="router.push('/pages/edit/0')">新建页面</el-button>
        <el-button @click="runGenerate('full')">全量生成</el-button>
        <el-button @click="runGenerate('increment')">增量生成</el-button>
        <el-button @click="createBackup">创建备份</el-button>
        <el-button @click="runRebuild">重建索引</el-button>
      </div>
    </div>

    <div class="card">
      <h3 style="margin: 0 0 12px; font-size: 15px">最近任务</h3>
      <el-table :data="stats?.recent_tasks || []" size="small">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="类型" width="120">
          <template #default="{ row }">{{ taskTypeText(row.type) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : 'info'">
              {{ statusText[row.status] || row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进度" width="120">
          <template #default="{ row }">
            <el-progress :percentage="row.progress || 0" :stroke-width="8" />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="结束时间" min-width="160">
          <template #default="{ row }">{{ row.finished_at ? formatDate(row.finished_at) : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="taskId = row.id; dialogVisible = true">查看</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <TaskProgressDialog v-model="dialogVisible" :task-id="taskId" @done="onTaskDone" />
  </div>
</template>
