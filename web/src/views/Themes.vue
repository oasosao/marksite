<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { themeApi } from '../api'
import type { ThemeInfo } from '../types'
import { emit } from '../utils/bus'

const list = ref<ThemeInfo[]>([])
const loading = ref(false)
const uploading = ref(false)
const fileRef = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    list.value = await themeApi.list()
  } finally {
    loading.value = false
  }
}

async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.zip')) {
    ElMessage.error('请上传 .zip 格式的主题包')
    return
  }
  if (file.size > 20 * 1024 * 1024) {
    ElMessage.error('主题包不能超过 20MB')
    return
  }
  uploading.value = true
  try {
    await themeApi.upload(file)
    ElMessage.success('主题已安装')
    load()
  } catch (err: any) {
    ElMessage.error(err?.message || '主题安装失败')
  } finally {
    uploading.value = false
  }
}

async function activate(theme: ThemeInfo) {
  await ElMessageBox.confirm(`确定激活主题「${theme.name}」吗？激活后需重新生成站点生效。`, '激活主题', {
    confirmButtonText: '激活', cancelButtonText: '取消',
  })
  await themeApi.activate(theme.name)
  ElMessage.success('已激活，请重新生成站点')
  emit('site-refresh')
  load()
}

async function remove(theme: ThemeInfo) {
  if (theme.builtin) {
    ElMessage.warning('内置主题不可删除')
    return
  }
  if (theme.active) {
    ElMessage.warning('请先切换到其他主题再删除')
    return
  }
  await ElMessageBox.confirm(`确定删除主题「${theme.name}」吗？`, '删除主题', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  await themeApi.remove(theme.name)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="page-header">
      <div>
        <h2 class="page-title">主题管理</h2>
        <p class="page-subtitle">主题包需包含 theme.json 与 index/post/page/archive 四个模板</p>
      </div>
      <div>
        <el-button type="primary" :loading="uploading" @click="fileRef?.click()">上传主题 zip</el-button>
        <input ref="fileRef" type="file" accept=".zip" style="display: none" @change="onFileChange" />
      </div>
    </div>

    <div class="theme-grid">
      <div v-for="theme in list" :key="theme.name" class="card">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px">
          <span style="font-weight: 600">{{ theme.name }}</span>
          <div style="display: flex; gap: 6px">
            <el-tag v-if="theme.builtin" size="small" type="info">内置</el-tag>
            <el-tag v-if="theme.active" size="small" type="success">当前</el-tag>
            <el-tag v-if="!theme.valid" size="small" type="danger">无效</el-tag>
          </div>
        </div>
        <div class="muted">版本 {{ theme.version || '-' }} · 作者 {{ theme.author || '-' }}</div>
        <div class="muted" style="margin: 6px 0 12px; min-height: 34px">{{ theme.description || '暂无描述' }}</div>
        <div style="display: flex; gap: 8px">
          <el-button size="small" type="primary" :disabled="theme.active || !theme.valid" @click="activate(theme)">激活</el-button>
          <el-button size="small" type="danger" text :disabled="theme.builtin || theme.active" @click="remove(theme)">删除</el-button>
        </div>
      </div>
      <div v-if="!loading && !list.length" class="muted">暂无主题</div>
    </div>
  </div>
</template>
