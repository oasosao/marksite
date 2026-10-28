<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { assetApi } from '../api'
import type { Asset } from '../types'
import { formatSize } from '../utils/format'

const props = defineProps<{ modelValue: boolean; multiple?: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'select', asset: Asset | Asset[]): void
  (e: 'openAssets'): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})

const list = ref<Asset[]>([])
const total = ref(0)
const loading = ref(false)
const uploading = ref(false)
const page = ref(1)
const size = 24
const selected = ref<Asset | null>(null)
const selectedAssets = ref<Map<number, Asset>>(new Map())
const fileInput = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    const res = await assetApi.list({ page: page.value, page_size: size })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function pick(asset: Asset) {
  if (props.multiple) {
    if (selectedAssets.value.has(asset.id)) {
      selectedAssets.value.delete(asset.id)
    } else {
      selectedAssets.value.set(asset.id, asset)
    }
  } else {
    selected.value = asset
  }
}

function confirm() {
  if (props.multiple) {
    if (selectedAssets.value.size === 0) {
      ElMessage.warning('请选择至少一张图片')
      return
    }
    emit('select', Array.from(selectedAssets.value.values()))
  } else {
    if (!selected.value) {
      ElMessage.warning('请选择一张图片')
      return
    }
    emit('select', selected.value)
  }
  visible.value = false
}

async function uploadFile(file: File) {
  uploading.value = true
  try {
    const asset = await assetApi.upload(file)
    ElMessage.success('上传成功')
    await load()
    if (props.multiple) {
      selectedAssets.value.set(asset.id, asset)
    } else {
      selected.value = asset
    }
  } finally {
    uploading.value = false
  }
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) uploadFile(file)
  input.value = ''
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      selected.value = null
      selectedAssets.value = new Map()
      page.value = 1
      load()
    }
  },
  { immediate: true },
)
</script>

<template>
  <el-dialog v-model="visible" title="选择图片" width="760px">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px">
      <div>
        <el-button size="small" @click="fileInput?.click()">上传图片</el-button>
        <input
          ref="fileInput"
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp,image/x-icon"
          style="display: none"
          @change="onFileChange"
        />
        <span class="muted" style="margin-left: 8px" v-if="uploading">上传中…</span>
      </div>
      <a href="javascript:void(0)" @click="emit('openAssets'); visible = false">打开资源管理</a>
    </div>
    <div v-loading="loading" class="picker-grid">
      <div
        v-for="asset in list"
        :key="asset.id"
        class="picker-item"
        :class="{ selected: multiple ? selectedAssets.has(asset.id) : selected?.id === asset.id }"
        @click="pick(asset)"
      >
        <img :src="asset.url" :alt="asset.filename" />
        <div class="picker-meta">{{ asset.filename }} · {{ formatSize(asset.size) }}</div>
        <div v-if="multiple && selectedAssets.has(asset.id)" class="picker-check">✓</div>
      </div>
      <div v-if="!loading && !list.length" class="muted">暂无图片，请先上传</div>
    </div>
    <el-pagination
      v-if="total > size"
      v-model:current-page="page"
      :page-size="size"
      :total="total"
      layout="prev, pager, next"
      style="margin-top: 12px; justify-content: center"
      @current-change="load"
    />
    <template #footer>
      <span class="muted" v-if="multiple">已选 {{ selectedAssets.size }} 张</span>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :disabled="multiple ? selectedAssets.size === 0 : !selected" @click="confirm">
        使用所选图片
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.picker-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 10px;
  min-height: 120px;
}
.picker-item {
  border: 2px solid transparent;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  background: #f8fafc;
}
.picker-item.selected {
  border-color: var(--el-color-primary);
}
.picker-check {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--el-color-primary);
  color: #fff;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.picker-item {
  position: relative;
}
.picker-item img {
  width: 100%;
  height: 96px;
  object-fit: cover;
  display: block;
}
.picker-meta {
  font-size: 11px;
  color: #6b7280;
  padding: 6px 8px;
  word-break: break-all;
}
</style>
