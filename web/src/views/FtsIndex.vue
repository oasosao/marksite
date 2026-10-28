<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ftsApi } from '../api'
import type { SearchHit, Task } from '../types'
import { formatDate, formatSize } from '../utils/format'
import TaskProgressDialog from '../components/TaskProgressDialog.vue'

const router = useRouter()
const status = ref<{ exists: boolean; docs: number; size: number; index_path: string } | null>(null)
const loading = ref(false)
const query = ref('')
const searching = ref(false)
const hits = ref<SearchHit[]>([])
const took = ref(0)
const total = ref(0)
const searched = ref(false)
const taskVisible = ref(false)
const taskId = ref(0)

async function loadStatus() {
  loading.value = true
  try {
    status.value = await ftsApi.status()
  } finally {
    loading.value = false
  }
}

async function rebuild() {
  const res = await ftsApi.rebuild()
  taskId.value = res.task_id
  taskVisible.value = true
}

async function search() {
  if (!query.value.trim()) {
    ElMessage.warning('请输入关键词')
    return
  }
  searching.value = true
  try {
    const res = await ftsApi.search(query.value.trim())
    hits.value = res.list
    took.value = res.took_ms
    total.value = res.total
    searched.value = true
  } finally {
    searching.value = false
  }
}

function openHit(hit: SearchHit) {
  router.push(hit.type === 'post' ? `/posts/edit/${hit.id}` : `/pages/edit/${hit.id}`)
}

function onTaskDone(task: Task) {
  loadStatus()
  if (task.status === 'success') ElMessage.success('索引重建完成')
  else if (task.status === 'failed') ElMessage.error(task.error || '索引重建失败')
}

onMounted(loadStatus)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">搜索索引管理</h2>
        <p class="page-subtitle">基于 Bluge 的独立索引（./ftsindex），数据来源为 SQLite</p>
      </div>
      <div class="page-actions">
        <el-button @click="loadStatus">刷新状态</el-button>
        <el-button type="primary" @click="rebuild">重建索引</el-button>
      </div>
    </div>

    <div class="stat-grid stat-grid--fill" v-loading="loading">
      <div class="stat-card">
        <div class="stat-label">索引状态</div>
        <div class="stat-value">{{ status?.exists ? '已建立' : '不存在' }}</div>
        <div class="stat-extra mono">{{ status?.index_path }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">文档数量</div>
        <div class="stat-value">{{ status?.docs ?? 0 }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">索引体积</div>
        <div class="stat-value">{{ formatSize(status?.size || 0) }}</div>
      </div>
    </div>

    <div class="card" style="margin-bottom: 14px">
      <h3 style="margin: 0 0 10px; font-size: 15px">搜索测试</h3>
      <div class="filter-row">
        <el-input v-model="query" placeholder="输入关键词，回车检索" @keyup.enter="search" />
        <el-button type="primary" :loading="searching" @click="search">搜索</el-button>
      </div>
      <div class="muted" style="margin-top: 8px">中文按单字切分、英文按单词切分；多个关键词为“与”关系。</div>
    </div>

    <div class="card" v-if="searched">
      <div class="muted" style="margin-bottom: 10px">共 {{ total }} 条结果，耗时 {{ took }} ms</div>
      <el-table :data="hits" size="small">
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.type === 'post' ? 'primary' : 'info'">
              {{ row.type === 'post' ? '文章' : '页面' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="标题" min-width="200">
          <template #default="{ row }">
            <el-link type="primary" @click="openHit(row as SearchHit)">{{ row.title }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="snippet" label="摘要片段" min-width="300" />
        <el-table-column label="更新时间" width="170">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
      </el-table>
      <div v-if="!hits.length" class="muted" style="margin-top: 8px">未找到结果</div>
    </div>

    <TaskProgressDialog v-model="taskVisible" :task-id="taskId" @done="onTaskDone" />
  </div>
</template>
