<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { assetApi } from '../api'
import type { Asset } from '../types'
import { formatDate, formatSize } from '../utils/format'

const list = ref<Asset[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(24)
const keyword = ref('')
const loading = ref(false)
const uploading = ref(false)
const uploadIndex = ref(0)
const uploadTotal = ref(0)
const fileRef = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    const res = await assetApi.list({ page: page.value, page_size: pageSize.value, keyword: keyword.value || undefined })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function uploadFiles(files: FileList | File[]) {
  const arr = Array.from(files)
  if (!arr.length) return
  uploading.value = true
  uploadTotal.value = arr.length
  uploadIndex.value = 0
  let ok = 0
  try {
    for (const file of arr) {
      uploadIndex.value++
      try {
        await assetApi.upload(file)
        ok++
      } catch {
        // 错误已由 http 层弹出 ElMessage，继续上传其余
      }
    }
    if (ok === arr.length) {
      ElMessage.success(`已上传 ${ok} 个文件`)
    } else {
      ElMessage.warning(`上传完成：成功 ${ok}，失败 ${arr.length - ok}`)
    }
    page.value = 1
    await load()
  } finally {
    uploading.value = false
  }
}

function onInputChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files?.length) uploadFiles(input.files)
  input.value = ''
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer?.files?.length) uploadFiles(e.dataTransfer.files)
}

async function copyPath(asset: Asset) {
  const url = `${window.location.origin}${asset.url}`
  await navigator.clipboard.writeText(url)
  ElMessage.success('已复制访问地址')
}

async function removeAsset(asset: Asset) {
  if (asset.ref_count > 0) {
    ElMessage.warning('该资源正在被引用，无法删除')
    return
  }
  await ElMessageBox.confirm(`确定删除「${asset.filename}」吗？`, '删除资源', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  await assetApi.remove(asset.id)
  ElMessage.success('已删除')
  load()
}

async function cleanUnused() {
  await ElMessageBox.confirm('将删除所有未被引用的资源文件，操作不可恢复，确定继续？', '清理未引用资源', {
    type: 'warning', confirmButtonText: '清理', cancelButtonText: '取消',
  })
  const res = await assetApi.cleanUnused()
  ElMessage.success(`扫描 ${res.scanned} 个，删除 ${res.deleted_count} 个，释放 ${formatSize(res.freed_bytes)}`)
  load()
}

function search() {
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h2 class="page-title">资源管理</h2>
        <p class="page-subtitle">上传图片附件并查看引用关系</p>
      </div>
      <div class="page-actions">
        <el-button @click="cleanUnused">清理未引用资源</el-button>
        <el-button type="primary" @click="fileRef?.click()">上传图片</el-button>
        <input
          ref="fileRef"
          type="file"
          multiple
          accept="image/png,image/jpeg,image/gif,image/webp,image/x-icon"
          style="display: none"
          @change="onInputChange"
        />
      </div>
    </div>

    <div
      class="card"
      style="margin-bottom: 16px; border: 1px dashed #cbd5e1; text-align: center; color: #6b7280"
      @dragover.prevent
      @drop="onDrop"
    >
      <div style="padding: 18px">
        <div style="font-size: 26px; margin-bottom: 6px">⬆️</div>
        <div>拖拽图片到此处上传，或点击上方「上传图片」按钮选择文件</div>
        <div class="muted" style="margin-top: 4px">支持 png / jpg / jpeg / gif / webp / ico，单文件不超过 20MB</div>
        <div v-if="uploading" class="muted" style="margin-top: 6px">
          正在上传 {{ uploadIndex }} / {{ uploadTotal }}…
        </div>
      </div>
    </div>

    <div class="card">
      <div class="filter-row" style="margin-bottom: 14px">
        <el-input v-model="keyword" placeholder="搜索文件名或路径" clearable @keyup.enter="search" />
        <el-button type="primary" plain @click="search">查询</el-button>
      </div>

      <div v-loading="loading" class="asset-grid">
        <div v-for="asset in list" :key="asset.id" class="asset-card">
          <img class="asset-thumb" :src="asset.url" :alt="asset.filename" />
          <div class="asset-meta">
            <div style="color: #111827">{{ asset.filename }}</div>
            <div>{{ formatSize(asset.size) }} · {{ formatDate(asset.created_at) }}</div>
            <div class="mono">{{ asset.path }}</div>
            <el-tag size="small" :type="asset.ref_count > 0 ? 'success' : 'info'" style="margin-top: 6px">
              引用 {{ asset.ref_count }}
            </el-tag>
          </div>
          <div class="asset-actions">
            <el-button size="small" @click="copyPath(asset)">复制地址</el-button>
            <el-button size="small" type="danger" text @click="removeAsset(asset)">删除</el-button>
          </div>
        </div>
        <div v-if="!loading && !list.length" class="muted" style="grid-column: 1 / -1">暂无资源</div>
      </div>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[12, 24, 48, 96]"
        layout="total, sizes, prev, pager, next"
        style="margin-top: 16px; justify-content: flex-end"
        @current-change="load"
        @size-change="search"
      />
    </div>
  </div>
</template>
