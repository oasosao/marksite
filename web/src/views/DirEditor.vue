<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import Vditor from 'vditor'
import 'vditor/dist/index.css'
import { assetApi, treeApi } from '../api'
import type { Asset } from '../types'
import { emit } from '../utils/bus'
import AssetPickerDialog from '../components/AssetPickerDialog.vue'

const route = useRoute()
const router = useRouter()
const id = computed(() => Number(route.params.id || 0))

const form = ref({
  name: '',
  slug: '',
  is_page: 0,
  is_published: 1,
  menu_visible: 1,
  page_title: '',
  page_summary: '',
  cover: '',
})
const content = ref('')
const loading = ref(false)
const saving = ref(false)
const pickerVisible = ref(false)
const vditorEl = ref<HTMLDivElement | null>(null)
let vd: Vditor | null = null

const urlPreview = computed(() => {
  const slug = form.value.slug || 'slug'
  if (form.value.is_page === 1) return `/${slug}.html`
  return `/${slug}/index.html`
})

function initVditor() {
  if (!vditorEl.value) return
  vd = new Vditor(vditorEl.value, {
    mode: 'sv',
    height: 480,
    cdn: '/vditor',
    cache: { enable: false },
    placeholder: '独立页面的正文内容（Markdown）…',
    toolbar: [
      'headings', 'bold', 'italic', 'strike', '|',
      'list', 'ordered-list', 'check', '|',
      'quote', 'line', 'code', 'inline-code', 'link', 'table', '|',
      'upload', 'undo', 'redo', '|', 'edit-mode', 'fullscreen',
    ],
    input: (value: string) => {
      content.value = value
    },
    after: () => {
      if (content.value) vd?.setValue(content.value)
    },
    upload: {
      handler: async (files: File[]) => {
        if (!files.length) return null
        const asset = await assetApi.upload(files[0])
        vd?.insertValue(`![${asset.filename}](${asset.url})`)
        return null
      },
    },
  })
}

async function load() {
  if (!id.value) return
  loading.value = true
  try {
    const dir = await treeApi.getDir(id.value)
    form.value = {
      name: dir.name,
      slug: dir.slug,
      is_page: dir.is_page,
      is_published: dir.is_published,
      menu_visible: dir.menu_visible,
      page_title: dir.page_title,
      page_summary: dir.page_summary,
      cover: dir.cover,
    }
    content.value = dir.page_content || ''
    if (vd) vd.setValue(content.value)
  } finally {
    loading.value = false
  }
}

function validate(): boolean {
  if (!form.value.name.trim()) {
    ElMessage.warning('名称不能为空')
    return false
  }
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(form.value.slug)) {
    ElMessage.warning('slug 仅允许小写字母、数字与中划线')
    return false
  }
  return true
}

async function save() {
  if (!validate()) return
  saving.value = true
  try {
    await treeApi.updateDir(id.value, {
      name: form.value.name.trim(),
      slug: form.value.slug.trim(),
      is_page: form.value.is_page,
      is_published: form.value.is_published,
      menu_visible: form.value.menu_visible,
      page_title: form.value.page_title,
      page_summary: form.value.page_summary,
      cover: form.value.cover,
      page_content: vd ? vd.getValue() : content.value,
    })
    ElMessage.success('已保存')
    emit('tree-refresh')
    emit('content-refresh')
  } finally {
    saving.value = false
  }
}

function onCoverSelected(asset: Asset | Asset[]) {
  const a = Array.isArray(asset) ? asset[0] : asset
  if (a) form.value.cover = a.path
}

function back() {
  router.back()
}

onMounted(async () => {
  await load()
  await nextTick()
  initVditor()
})

onBeforeUnmount(() => {
  vd?.destroy()
  vd = null
})
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="page-header">
      <div style="display: flex; align-items: center; gap: 10px">
        <el-button text @click="back">← 返回</el-button>
        <h2 class="page-title">编辑分类 / 独立页面</h2>
      </div>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </div>

    <div class="card" style="margin-bottom: 14px">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="名称" required>
              <el-input v-model="form.name" placeholder="分类名称" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="slug" required>
              <el-input v-model="form.slug" placeholder="小写字母、数字、中划线" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="类型">
              <el-radio-group v-model="form.is_page">
                <el-radio :value="0">分类（可添加文章）</el-radio>
                <el-radio :value="1">独立页面</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="生成路径">
              <span class="muted">{{ urlPreview }}</span>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="发布">
              <el-switch v-model="form.is_published" :active-value="1" :inactive-value="0" active-text="已发布" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="导航菜单">
              <el-switch
                v-model="form.menu_visible"
                :active-value="1"
                :inactive-value="0"
                active-text="显示在导航"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="页面标题">
          <el-input v-model="form.page_title" placeholder="留空则使用分类名称" />
        </el-form-item>
        <el-form-item label="页面摘要">
          <el-input v-model="form.page_summary" type="textarea" :rows="2" placeholder="页面摘要（可选）" />
        </el-form-item>
        <el-form-item label="封面">
          <div style="display: flex; align-items: center; gap: 8px; width: 100%">
            <el-input v-model="form.cover" placeholder="资源 path（可选）" readonly />
            <el-button @click="pickerVisible = true">选择</el-button>
            <el-button v-if="form.cover" text @click="form.cover = ''">清除</el-button>
          </div>
        </el-form-item>
      </el-form>
    </div>

    <div class="card" style="margin-bottom: 10px">
      <p v-if="form.is_page === 0" class="muted" style="margin: 0">
        当前为分类模式，下方页面内容不会参与生成；切换为“独立页面”后生效。
      </p>
      <p v-else class="muted" style="margin: 0">独立页面正文（生成到 {{ urlPreview }}）</p>
    </div>

    <div class="card" style="padding: 0">
      <div ref="vditorEl" class="vditor-host" />
    </div>

    <AssetPickerDialog v-model="pickerVisible" @select="onCoverSelected" @open-assets="router.push('/assets')" />
  </div>
</template>
