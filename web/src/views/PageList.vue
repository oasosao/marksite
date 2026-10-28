<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { pageApi, treeApi } from '../api'
import type { Page, TreeNode } from '../types'
import { flattenDirs } from '../utils/dirTree'
import { formatDate } from '../utils/format'
import { off, on } from '../utils/bus'

const router = useRouter()
const rows = ref<Page[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const published = ref('')
const selection = ref<Page[]>([])

const treeData = ref<TreeNode[]>([])
const dirOptions = computed(() => [{ value: 0, label: '根分类' }, ...flattenDirs(treeData.value).map((d) => ({
  value: d.value, label: `${'\u00a0\u00a0'.repeat(d.depth)}${d.label}`,
}))])

const importDialog = ref(false)
const importDir = ref(0)
const importFile = ref<File | null>(null)
const importing = ref(false)
const fileRef = ref<HTMLInputElement | null>(null)

async function loadTree() {
  treeData.value = await treeApi.get()
}

async function load() {
  loading.value = true
  try {
    const res = await pageApi.list({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      is_published: published.value === '' ? undefined : published.value,
    })
    rows.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function onSelectionChange(list: Page[]) {
  selection.value = list
}

async function removeRow(row: Page) {
  await ElMessageBox.confirm(`确定删除「${row.title}」吗？`, '删除确认', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  await pageApi.remove(row.id)
  ElMessage.success('已删除')
  load()
}

async function exportRow(row: Page) {
  await pageApi.exportOne(row.id, `${row.slug}.md`)
}

async function batchExport() {
  if (!selection.value.length) {
    ElMessage.warning('请先勾选页面')
    return
  }
  await pageApi.batchExport(selection.value.map((r) => r.id))
}

function openImport() {
  importDir.value = 0
  importFile.value = null
  importDialog.value = true
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  importFile.value = input.files?.[0] || null
}

async function doImport() {
  if (!importFile.value) {
    ElMessage.warning('请选择 .md 或 .zip 文件')
    return
  }
  importing.value = true
  try {
    const res = await pageApi.importZip(importFile.value, importDir.value)
    importDialog.value = false
    ElMessage.success(`导入完成：成功 ${res.imported}，跳过 ${res.skipped}`)
    loadTree()
    load()
    onTreeChanged()
  } finally {
    importing.value = false
  }
}

function onTreeChanged() {
  loadTree()
}

onMounted(() => {
  loadTree()
  load()
  on('content-refresh', onTreeChanged)
})
onBeforeUnmount(() => off('content-refresh', onTreeChanged))
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">页面列表</h2>
        <p class="page-subtitle">独立页面（不进列表、归档与 RSS）</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="router.push('/pages/edit/0')">新建页面</el-button>
        <el-button @click="openImport">导入页面</el-button>
        <el-button :disabled="!selection.length" @click="batchExport">批量导出</el-button>
      </div>
    </div>

    <div class="card" style="margin-bottom: 14px">
      <div class="filter-row">
        <el-input v-model="keyword" placeholder="搜索标题" clearable @keyup.enter="search" />
        <el-select v-model="published" placeholder="发布状态" clearable @change="search">
          <el-option label="已发布" value="1" />
          <el-option label="未发布" value="0" />
        </el-select>
        <el-button type="primary" plain @click="search">查询</el-button>
      </div>
    </div>

    <div class="card">
      <el-table v-loading="loading" :data="rows" size="small" @selection-change="onSelectionChange">
        <el-table-column type="selection" width="44" />
        <el-table-column label="标题" min-width="240">
          <template #default="{ row }">
            <el-link type="primary" @click="router.push(`/pages/edit/${row.id}`)">{{ row.title }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="slug" label="slug" min-width="160" />
        <el-table-column label="发布" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.is_published ? 'success' : 'info'">
              {{ row.is_published ? '已发布' : '草稿' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column label="更新时间" width="170">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="190" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="router.push(`/pages/edit/${row.id}`)">编辑</el-button>
            <el-button size="small" text @click="exportRow(row as Page)">导出</el-button>
            <el-button size="small" text type="danger" @click="removeRow(row as Page)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        style="margin-top: 14px; justify-content: flex-end"
        @current-change="load"
        @size-change="search"
      />
    </div>

    <el-dialog v-model="importDialog" title="导入页面" width="460px">
      <el-form label-width="90px">
        <el-form-item label="目标分类">
          <el-tree-select v-model="importDir" :data="dirOptions" check-strictly :render-after-expand="false" style="width: 100%" />
        </el-form-item>
        <el-form-item label="文件">
          <el-button size="small" @click="fileRef?.click()">选择 .md 或 .zip</el-button>
          <input ref="fileRef" type="file" accept=".md,.zip" style="display: none" @change="onFileChange" />
          <span class="muted" style="margin-left: 8px">{{ importFile?.name || '未选择' }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialog = false">取消</el-button>
        <el-button type="primary" :loading="importing" @click="doImport">开始导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>
