<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import Vditor from 'vditor'
import 'vditor/dist/index.css'
import { assetApi, pageApi, postApi, tagApi, treeApi } from '../api'
import type { Asset, Page, Post, SnapshotMeta, TagCount, TreeNode } from '../types'
import { flattenDirs } from '../utils/dirTree'
import { formatDate, slugify, toLocalInput, toRFC3339 } from '../utils/format'
import { emit } from '../utils/bus'
import AssetPickerDialog from './AssetPickerDialog.vue'

const props = defineProps<{ kind: 'post' | 'page' }>()

const router = useRouter()
const route = useRoute()
const id = computed(() => Number(route.params.id || 0))
const isNew = computed(() => id.value === 0)
const api = computed(() => (props.kind === 'post' ? postApi : pageApi))

async function fetchContent(id: number): Promise<Post | Page> {
  return props.kind === 'post' ? postApi.get(id) : pageApi.get(id)
}

const form = ref({
  title: '',
  slug: '',
  dir_id: 0,
  tags: [] as string[],
  summary: '',
  cover: '',
  gallery: [] as string[],
  sort: 0,
  is_published: 0,
  published_at_local: '',
})
const content = ref('')
const saving = ref(false)
const loading = ref(false)

const treeData = ref<TreeNode[]>([])
const dirOptions = computed(() => [{ value: 0, label: '根分类' }, ...flattenDirs(treeData.value).map((d) => ({
  value: d.value, label: `${'\u00a0\u00a0'.repeat(d.depth)}${d.label}`,
}))])

const snapshots = ref<SnapshotMeta[]>([])
const tagOptions = ref<TagCount[]>([])
const pickerVisible = ref(false)
const pickerMode = ref<'cover' | 'gallery'>('cover')
const vditorEl = ref<HTMLDivElement | null>(null)
const metaCollapsed = ref(false)     // 窄屏 meta-panel 默认折叠，让出主空间
let vd: Vditor | null = null

const title = computed(() => (props.kind === 'post' ? (isNew.value ? '新建文章' : '编辑文章') : isNew.value ? '新建页面' : '编辑页面'))

async function loadTree() {
  treeData.value = await treeApi.get()
}

async function loadTags() {
  tagOptions.value = await tagApi.list()
}

async function loadContent() {
  if (isNew.value) {
    const q = Number(route.query.dir_id || 0)
    form.value.dir_id = Number.isFinite(q) ? q : 0
    return
  }
  loading.value = true
  try {
    const data = await fetchContent(id.value)
    form.value = {
      title: data.title,
      slug: data.slug,
      dir_id: data.dir_id,
      tags: data.tags ? data.tags.map((t) => t.name) : [],
      summary: data.summary,
      cover: data.cover,
      gallery: data.gallery ? [...data.gallery] : [],
      sort: data.sort,
      is_published: data.is_published,
      published_at_local: toLocalInput(data.published_at),
    }
    content.value = data.content
    if (vd) vd.setValue(data.content)
    await loadSnapshots()
  } finally {
    loading.value = false
  }
}

async function loadSnapshots() {
  if (isNew.value) {
    snapshots.value = []
    return
  }
  snapshots.value = await api.value.snapshots(id.value)
}

function initVditor() {
  if (!vditorEl.value) return
  vd = new Vditor(vditorEl.value, {
    // 窄屏下分屏只剩一条窄缝，改用「即时渲染」单栏；宽屏仍默认分屏预览
    mode: 'wysiwyg',
    height: '100%',
    cdn: '/vditor',
    cache: { enable: false },
    placeholder: '在此输入 Markdown 正文…',
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

function openCoverPicker() {
  pickerMode.value = 'cover'
  pickerVisible.value = true
}

function openGalleryPicker() {
  pickerMode.value = 'gallery'
  pickerVisible.value = true
}

function onAssetSelected(asset: Asset | Asset[]) {
  if (Array.isArray(asset)) {
    // 图集多选：追加去重
    const exist = new Set(form.value.gallery)
    for (const a of asset) {
      if (!exist.has(a.path)) {
        form.value.gallery.push(a.path)
        exist.add(a.path)
      }
    }
  } else {
    form.value.cover = asset.path
  }
}

function clearCover() {
  form.value.cover = ''
}

function removeGalleryItem(index: number) {
  form.value.gallery.splice(index, 1)
}

function moveGalleryItem(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= form.value.gallery.length) return
  const arr = form.value.gallery
  ;[arr[index], arr[target]] = [arr[target], arr[index]]
}

function clearGallery() {
  form.value.gallery = []
}

function onTitleInput(value: string) {
  if (isNew.value && !form.value.slug) {
    form.value.slug = slugify(value)
  }
}

function validate(): boolean {
  if (!form.value.title.trim()) {
    ElMessage.warning('标题不能为空')
    return false
  }
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(form.value.slug)) {
    ElMessage.warning('slug 仅允许小写字母、数字与中划线')
    return false
  }
  return true
}

function payload() {
  return {
    dir_id: form.value.dir_id,
    title: form.value.title.trim(),
    slug: form.value.slug.trim(),
    content: vd ? vd.getValue() : content.value,
    summary: form.value.summary,
    cover: form.value.cover,
    gallery: form.value.gallery,
    tags: form.value.tags,
    sort: Number(form.value.sort) || 0,
    is_published: form.value.is_published,
    published_at: toRFC3339(form.value.published_at_local),
  }
}

async function save() {
  if (!validate()) return
  saving.value = true
  try {
    if (isNew.value) {
      const created = await api.value.create(payload())
      ElMessage.success('已创建')
      emit('tree-refresh')
      emit('content-refresh')
      // 保留原始 query（例如 dir_id），以便保存后仍在原分类上下文
      await router.replace({
        path: `/${props.kind === 'post' ? 'posts' : 'pages'}/edit/${created.id}`,
        query: route.query,
      })
      await loadContent()
    } else {
      await api.value.update(id.value, payload())
      ElMessage.success('已保存')
      emit('tree-refresh')
      emit('content-refresh')
      await loadSnapshots()
    }
  } finally {
    saving.value = false
  }
}

async function exportOne() {
  if (isNew.value) return
  await api.value.exportOne(id.value, `${form.value.slug}.md`)
}

async function rollback(snapshot: SnapshotMeta) {
  await ElMessageBox.confirm(
    `确定回滚到快照「${snapshot.title}」（${formatDate(snapshot.created_at)}）吗？当前版本会先保存为新快照。`,
    '回滚确认',
    { type: 'warning', confirmButtonText: '回滚', cancelButtonText: '取消' },
  )
  await api.value.rollback(id.value, snapshot.id)
  ElMessage.success('已回滚')
  emit('tree-refresh')
  emit('content-refresh')
  await loadContent()
}

function back() {
  router.push(props.kind === 'post' ? '/posts' : '/pages')
}

const handleResize = () => {
  if (window.innerWidth > 1280) metaCollapsed.value = false
}

onMounted(async () => {
  await loadTree()
  await loadTags()
  await loadContent()
  await nextTick()
  initVditor()
  metaCollapsed.value = window.innerWidth <= 1280
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  vd?.destroy()
  vd = null
})
</script>

<template>
  <div class="editor-page" v-loading="loading">
    <div class="page-header">
      <div style="display: flex; align-items: center; gap: 10px">
        <el-button text @click="back">← 返回列表</el-button>
        <h2 class="page-title">{{ title }}</h2>
      </div>
      <div class="page-header-actions">
        <el-button v-if="!isNew" @click="exportOne">导出 Markdown</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </div>
    </div>
    <!-- 折叠态占位条 + 打开按钮 -->
    <div v-if="metaCollapsed" class="meta-opener">
      <span class="meta-opener-title">标题：{{ form.title || '未命名内容' }}</span>
      <span class="meta-opener-summary">
        slug: {{ form.slug || '（保存后自动生成）' }}
        {{ form.is_published ? '· 已发布' : '· 草稿' }}
        {{ form.tags.length ? `· ${form.tags.length} 个标签` : '' }}
      </span>
      <el-button size="small" type="primary" plain  @click="metaCollapsed = false">⚙ 设置</el-button>
    </div>
    <div class="editor-body">
      <div class="editor-panel card">
        <div ref="vditorEl" class="vditor-host"></div>
        <el-collapse v-if="!isNew">
          <el-collapse-item title="历史快照" name="snapshots">
            <el-table :data="snapshots" size="small" max-height="200">
              <el-table-column prop="id" label="ID" width="70" />
              <el-table-column prop="title" label="标题" min-width="220" />
              <el-table-column label="保存时间" width="180">
                <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
              </el-table-column>
              <el-table-column label="操作" width="100">
                <template #default="{ row }">
                  <el-button size="small" text type="warning" @click="rollback(row as SnapshotMeta)">回滚</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-collapse-item>
        </el-collapse>
      </div>

      <!-- 右侧 -->
      <div v-show="!metaCollapsed" class="meta-panel card" :class="{ 'is-open': !metaCollapsed }">
        <div class="meta-header">
          <span>内容设置</span>
          <el-button size="small" text type="primary" @click="metaCollapsed = true">收起</el-button>
        </div>
        <el-form label-width="90px">
          <el-form-item label="标题" required>
            <el-input v-model="form.title" placeholder="内容标题" @input="onTitleInput" />
          </el-form-item>
          <el-form-item label="slug" required>
            <el-input v-model="form.slug" placeholder="小写字母、数字、中划线" />
          </el-form-item>
          <el-form-item label="分类">
            <el-tree-select
              v-model="form.dir_id"
              :data="dirOptions"
              check-strictly
              :render-after-expand="false"
              placeholder="根分类"
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item label="标签">
            <el-select
              v-model="form.tags"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="选择或输入后回车新建"
              style="width: 100%"
            >
              <el-option v-for="t in tagOptions" :key="t.id" :label="t.name" :value="t.name">
                <span>{{ t.name }}</span>
                <span class="tag-slug">/{{ t.slug }}</span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="摘要">
            <el-input v-model="form.summary" type="textarea" :rows="2" placeholder="列表页摘要" />
          </el-form-item>
          <el-form-item label="封面">
            <div style="display: flex; align-items: center; gap: 8px; width: 100%">
              <el-input v-model="form.cover" placeholder="资源 path（可选）" readonly />
              <el-button @click="openCoverPicker">选择</el-button>
              <el-button v-if="form.cover" text @click="clearCover">清除</el-button>
            </div>
          </el-form-item>
          <el-form-item v-if="form.cover" label="封面预览">
            <img :src="`/media/${form.cover}`" alt="封面预览" style="height: 60px; border-radius: 6px; border: 1px solid #eef0f4" />
          </el-form-item>
          <el-form-item label="图集">
            <div class="gallery-block">
              <div class="gallery-toolbar">
                <el-button size="small" type="primary" plain @click="openGalleryPicker">从资源库添加</el-button>
                <el-button v-if="form.gallery.length" size="small" text type="danger" @click="clearGallery">清空</el-button>
                <span class="muted" style="margin-left: auto">共 {{ form.gallery.length }} 张</span>
              </div>
              <div v-if="form.gallery.length" class="gallery-grid">
                <div v-for="(path, idx) in form.gallery" :key="path" class="gallery-item">
                  <img :src="`/media/${path}`" :alt="`图 ${idx + 1}`" />
                  <div class="gallery-actions">
                    <el-button size="small" text @click="moveGalleryItem(idx, -1)" :disabled="idx === 0">↑</el-button>
                    <el-button size="small" text @click="moveGalleryItem(idx, 1)" :disabled="idx === form.gallery.length - 1">↓</el-button>
                    <el-button size="small" text type="danger" @click="removeGalleryItem(idx)">删除</el-button>
                  </div>
                </div>
              </div>
              <div v-else class="muted" style="font-size: 12px">暂未添加图集图片</div>
            </div>
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="form.sort" :min="0" :max="99999" controls-position="right" />
          </el-form-item>
          <el-form-item label="发布">
            <el-switch v-model="form.is_published" :active-value="1" :inactive-value="0" active-text="发布" />
          </el-form-item>
          <el-form-item label="发布时间">
            <el-date-picker
              v-model="form.published_at_local"
              type="datetime"
              placeholder="留空则发布时自动填写"
              value-format="YYYY-MM-DDTHH:mm"
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item v-if="form.cover" label="封面预览">
            <img :src="`/media/${form.cover}`" alt="封面预览" style="height: 60px; border-radius: 6px; border: 1px solid #eef0f4" />
          </el-form-item>
        </el-form>
      </div>

    </div>

    <AssetPickerDialog v-model="pickerVisible" :multiple="pickerMode === 'gallery'" @select="onAssetSelected" @open-assets="router.push('/assets')" />
  </div>
</template>

<style scoped>
/* 让整页撑满 .main（calc 减去 header 高度 + padding） */
.editor-page {
  height: calc(100vh);
  padding: 12px 20px 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.editor-body {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  gap: 16px;
}

.meta-panel {
  flex: 0 0 480px;
  width: 480px;
  overflow-y: auto;
  padding: 10px 14px 12px;
  border-radius: 10px;
}

.meta-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  font-size: 13px;
  color: #4b5563;
  padding-bottom: 6px;
  border-bottom: 1px solid #eef0f4;
  margin-bottom: 6px;
}

.meta-opener {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
  cursor: pointer;
  margin-bottom: 10px;
}
.meta-opener:hover { border-color: #c4cbd6; }
.meta-opener-title {
  font-weight: 600;
  font-size: 14px;
  min-width: 120px;
}
.meta-opener-summary {
  flex: 1 1 auto;
  font-size: 12px;
  color: #6b7280;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 把卡片的 padding/圆角 重置一下，因为要嵌进 flex 布局 */
.meta-panel.card { box-shadow: 0 1px 2px rgba(16, 24, 40, 0.06); }

.meta-panel :deep(.el-form-item) {
  margin-bottom: 12px;
}
.meta-panel :deep(.el-form-item__label) {
  width: 72px !important;
  font-size: 13px;
}
.meta-panel :deep(.el-form-item__content) {
  width: calc(100% - 72px);
}

.editor-panel {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 12px;
  overflow: hidden;
}

.vditor-host {
  flex: 1 1 auto;
  min-height: 360px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 10px;
}

/* Vditor 默认 520px 行内高度会撑死容器，强制覆盖 */
.vditor-host :deep(.vditor) {
  height: 100% !important;
}
.vditor-host :deep(.vditor-reset) {
  height: calc(100% - 44px);
  overflow-y: auto;
}

.editor-snapshots {
  margin-top: 10px;
  padding: 4px 0;
  max-height: 240px;
  overflow-y: auto;
}

.page-header-actions {
  display: flex;
  gap: 8px;
}

.tag-slug {
  float: right;
  color: #a0a6b3;
  font-size: 12px;
}

.gallery-block {
  width: 100%;
}
.gallery-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.gallery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
  gap: 8px;
}
.gallery-item {
  position: relative;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid #e5e7eb;
  background: #f8fafc;
}
.gallery-item img {
  width: 100%;
  height: 80px;
  object-fit: cover;
  display: block;
}
.gallery-actions {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s;
}
.gallery-item:hover .gallery-actions {
  opacity: 1;
}
.gallery-actions .el-button {
  color: #fff;
  padding: 2px 4px;
}
.gallery-actions .el-button:disabled {
  color: rgba(255, 255, 255, 0.4);
}

/* 中等及以下屏幕：meta-panel 变成抽屉覆盖层，editor-panel 始终撑满剩余高度 */
@media (max-width: 1280px) {
  .editor-page {
    padding: 8px 12px;
    overflow: hidden;
  }
  .editor-body {
    flex: 1 1 auto;
    min-height: 0;
    flex-direction: column;
    gap: 10px;
    overflow: hidden;
  }

  /* 折叠条 */
  .meta-opener {
    flex: 0 0 auto;
    padding: 8px 12px;
    margin-bottom: 0;
  }

  .meta-panel {
    position: fixed;
    top: 56px;
    right: 0;
    bottom: 0;
    width: min(520px, 100vw - 20px);
    z-index: 60;
    border-radius: 12px 0 0 12px;
    box-shadow: -8px 0 24px rgba(16, 24, 40, 0.18);
    flex: 0 0 auto;
    max-height: none;
    padding: 14px 16px;
    overflow-y: auto;
  }

  /* meta-panel 打开后的背景蒙层 + body relative */
  .editor-body:has(.meta-panel.is-open) {
    position: relative;
  }
  .editor-body:has(.meta-panel.is-open)::after {
    content: '';
    position: fixed;
    top: 56px;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(16, 24, 40, 0.3);
    z-index: 55;
  }

  .editor-panel {
    flex: 1 1 auto;
    min-height: 0;
    padding: 10px;
    overflow: hidden;
  }
  .vditor-host,
  .vditor-host :deep(.vditor) {
    height: 100% !important;
  }
  .vditor-host :deep(.vditor-reset) {
    height: calc(100% - 44px) !important;
  }
}

@media (max-width: 640px) {
  .editor-page { padding: 6px 10px; height: calc(100vh - 56px - 12px); }
  .meta-opener-title { min-width: 80px; font-size: 13px; }
  .meta-opener-summary { display: none; }
  .meta-panel { width: 100vw; border-radius: 12px 12px 0 0; }
  .meta-panel :deep(.el-form-item__label) { width: 64px !important; font-size: 12px; }
  .meta-panel :deep(.el-form-item__content) { width: calc(100% - 64px); }
  .editor-snapshots { max-height: 140px; }
}
</style>
