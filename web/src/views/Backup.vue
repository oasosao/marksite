<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { backupApi } from '../api'
import type { BackupInfo } from '../types'
import { formatDate, formatSize } from '../utils/format'

const list = ref<BackupInfo[]>([])
const loading = ref(false)
const creating = ref(false)
const restoreDialog = ref(false)
const restoreTarget = ref<BackupInfo | null>(null)
const confirmText = ref('')
const restoring = ref(false)

async function load() {
  loading.value = true
  try {
    list.value = await backupApi.list()
  } finally {
    loading.value = false
  }
}

async function create() {
  creating.value = true
  try {
    const res = await backupApi.create()
    ElMessage.success(`备份已创建：${res.name}`)
    load()
  } finally {
    creating.value = false
  }
}

async function remove(row: BackupInfo) {
  await ElMessageBox.confirm(`确定删除备份「${row.name}」吗？`, '删除备份', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  await backupApi.remove(row.name)
  ElMessage.success('已删除')
  load()
}

function openRestore(row: BackupInfo) {
  restoreTarget.value = row
  confirmText.value = ''
  restoreDialog.value = true
}

async function doRestore() {
  if (!restoreTarget.value) return
  restoring.value = true
  try {
    const res = await backupApi.restore(restoreTarget.value.name)
    restoreDialog.value = false
    ElMessage.success(`恢复成功，已生成安全备份 ${res.safety_backup}。如管理员密码变更请重新登录。`)
    load()
  } finally {
    restoring.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">备份恢复</h2>
        <p class="page-subtitle">备份包含数据库、资源目录与搜索索引</p>
      </div>
      <div class="page-actions">
        <el-button @click="load">刷新</el-button>
        <el-button type="primary" :loading="creating" @click="create">创建备份</el-button>
      </div>
    </div>

    <div class="card">
      <el-table v-loading="loading" :data="list" size="small">
        <el-table-column prop="name" label="文件名" min-width="240" />
        <el-table-column label="大小" width="110">
          <template #default="{ row }">{{ formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.auto ? 'info' : 'success'">{{ row.auto ? '自动' : '手动' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="backupApi.download(row.name)">下载</el-button>
            <el-button size="small" text type="warning" @click="openRestore(row as BackupInfo)">恢复</el-button>
            <el-button size="small" text type="danger" @click="remove(row as BackupInfo)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="restoreDialog" title="恢复备份" width="520px">
      <div class="warn-box" style="background: #fef2f2; border-color: #fecaca; color: #991b1b">
        恢复将用备份覆盖当前数据库、资源目录与搜索索引，当前数据会被替换（系统会先自动生成一份安全备份）。生成/索引任务运行中时无法恢复。
      </div>
      <p class="muted">目标备份：{{ restoreTarget?.name }}</p>
      <el-input v-model="confirmText" placeholder="请输入「恢复」以确认" />
      <template #footer>
        <el-button @click="restoreDialog = false">取消</el-button>
        <el-button type="danger" :disabled="confirmText !== '恢复'" :loading="restoring" @click="doRestore">
          确认恢复
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
