<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { configApi } from '../api'
import type { AppConfig } from '../types'
import {
  desktopShowSites,
  desktopWorkDirInfo,
  isDesktop,
  type WorkDirInfo,
} from '../utils/desktop'

const form = ref<AppConfig | null>(null)
const loading = ref(false)
const saving = ref(false)
const workDir = ref<WorkDirInfo | null>(null)

async function load() {
  loading.value = true
  try {
    form.value = await configApi.getApp()
  } finally {
    loading.value = false
  }
}

async function loadWorkDir() {
  workDir.value = await desktopWorkDirInfo()
}

async function save() {
  if (!form.value) return
  saving.value = true
  try {
    form.value = await configApi.updateApp(form.value)
    ElMessage.success('已保存')
  } finally {
    saving.value = false
  }
}

// 返回站点管理页，重新选择或新建站点
async function manageSites() {
  try {
    await ElMessageBox.confirm(
      '将关闭当前站点并返回站点管理页，未保存的设置修改会丢失。',
      '返回站点管理',
      { type: 'warning', confirmButtonText: '返回', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  try {
    await desktopShowSites()
  } catch (e) {
    ElMessage.error(String(e) || '返回站点管理失败')
  }
}

onMounted(() => {
  load()
  loadWorkDir()
})
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="page-header">
      <div>
        <h2 class="page-title">程序设置</h2>
        <p class="page-subtitle">站点、生成与备份策略</p>
      </div>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </div>

    <div v-if="form">
      <div v-if="!isDesktop()" class="warn-box">
        修改「监听地址」与「自动打开浏览器」需重启程序后生效。
      </div>

      <div v-if="workDir" class="card" style="margin-bottom: 14px">
        <el-form label-width="160px">
          <el-form-item label="当前站点目录">
            <span class="path-text">{{ workDir.work_dir }}</span>
            <el-button size="small" style="margin-left: 12px" @click="manageSites">站点管理…</el-button>
          </el-form-item>
          <el-form-item label="源数据目录">
            <span class="path-text">{{ workDir.data_dir }}</span>
          </el-form-item>
          <el-form-item label="默认输出目录">
            <span class="path-text">{{ workDir.output_dir }}</span>
          </el-form-item>
          <el-form-item>
            <span class="muted">
              每个站点是一个工作目录：data/ 存放数据库、素材、主题、索引与备份，output/ 是默认的站点输出目录。通过「站点管理」可以在多个站点间切换或新建站点。
            </span>
          </el-form-item>
        </el-form>
      </div>

      <div class="card" style="margin-bottom: 14px">
        <el-form label-width="160px">
          <el-form-item label="日志级别">
            <el-select v-model="form.log_level" style="width: 200px">
              <el-option label="debug" value="debug" />
              <el-option label="info" value="info" />
              <el-option label="warn" value="warn" />
              <el-option label="error" value="error" />
            </el-select>
            <span class="muted" style="margin-left: 10px">保存立即生效</span>
          </el-form-item>
          <el-form-item v-if="!isDesktop()" label="监听地址">
            <el-input v-model="form.listen_addr" placeholder="127.0.0.1:8080" style="width: 280px" />
            <span class="muted" style="margin-left: 10px">host:port，重启生效</span>
          </el-form-item>
          <el-form-item v-if="!isDesktop()" label="自动打开浏览器">
            <el-switch v-model="form.auto_open_browser" :active-value="1" :inactive-value="0" />
            <span class="muted" style="margin-left: 10px">重启生效</span>
          </el-form-item>
        </el-form>
      </div>

      <div class="card" style="margin-bottom: 14px">
        <el-form label-width="160px">
          <el-form-item label="输出目录">
            <el-input
              v-model="form.output_dir"
              :placeholder="workDir ? workDir.output_dir : '留空则使用当前站点的 output 目录'"
              style="width: 320px"
            />
            <div class="muted" style="width: 100%; margin-top: 4px">
              留空表示输出到当前站点的 output 目录；也可填写其他绝对或相对路径。请勿填写系统根目录或程序目录。
            </div>
          </el-form-item>
          <el-form-item label="快照保留上限">
            <el-input-number v-model="form.snapshot_limit" :min="1" :max="200" controls-position="right" />
            <span class="muted" style="margin-left: 10px">每篇内容最多保留的历史版本数</span>
          </el-form-item>
        </el-form>
      </div>

      <div class="card">
        <el-form label-width="160px">
          <el-form-item label="自动备份">
            <el-switch v-model="form.auto_backup_enabled" :active-value="1" :inactive-value="0" />
          </el-form-item>
          <el-form-item label="备份间隔（小时）">
            <el-input-number
              v-model="form.auto_backup_interval_hours"
              :min="1"
              :max="168"
              :disabled="form.auto_backup_enabled === 0"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="自动备份保留数量">
            <el-input-number
              v-model="form.backup_keep_count"
              :min="1"
              :max="50"
              :disabled="form.auto_backup_enabled === 0"
              controls-position="right"
            />
          </el-form-item>
        </el-form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.path-text {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 13px;
  color: var(--ms-text-strong);
  word-break: break-all;
}
</style>
