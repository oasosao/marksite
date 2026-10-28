<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { pageApi, postApi, treeApi } from '../api'
import type { Page, Post, Tag, TreeNode } from '../types'
import { flattenDirs } from '../utils/dirTree'
import { formatDate } from '../utils/format'
import { off, on, emit } from '../utils/bus'

interface Row {
  type: 'post' | 'page'
  id: number
  dir_id: number
  title: string
  slug: string
  tags: Tag[]
  is_published: number
  sort: number
  updated_at: string
  dir_name?: string
  dir_path?: string
}

const router = useRouter()
const route = useRoute()

const dirID = computed(() => {
  const v = Number(route.query.dir_id || 0)
  return Number.isFinite(v) ? v : 0
})
const mixed = computed(() => dirID.value > 0)

// 子目录排序映射（从 treeData 构建），key=dir_id, value={name, sort}
const dirSortMap = computed(() => {
  const m = new Map<number, { name: string; sort: number }>()
  const walk = (nodes: TreeNode[], depth = 0) => {
    for (const n of nodes) {
      if (n.type === 'dir') m.set(n.id, { name: n.label, sort: (n.sort ?? 0) + depth * 1000 })
      walk(n.children || [], depth + 1)
    }
  }
  walk(treeData.value)
  return m
})

interface DirGroup { dir_id: number; dir_name: string; rows: Row[] }
// mixed 视图下：如果聚合了多个子目录 → 按子分类分组展示
// 注意：必须基于全量 rows 判断，不能用 pagedRows（分页切片可能只含单一子目录导致误判）
const groupMode = computed(() => {
  if (!mixed.value) return false
  const u = new Set<number>()
  for (const r of rows.value) u.add(((r as Row & { dirID?: number }).dir_id ?? 0))
  return u.size > 1
})
const groupedRows = computed<DirGroup[]>(() => {
  const m = new Map<number, Row[]>()
  for (const r of rows.value) {
    const id = (r as Row & { dirID?: number }).dir_id ?? 0
    const arr = m.get(id) || []
    arr.push(r)
    m.set(id, arr)
  }
  const groups: DirGroup[] = []
  for (const [id, rs] of m) {
    const info = dirSortMap.value.get(id)
    groups.push({
      dir_id: id,
      dir_name: info?.name ?? rs[0].dir_name ?? (id === 0 ? '未分类' : String(id)),
      rows: rs,
    })
  }
  groups.sort((a, b) => {
    const sa = dirSortMap.value.get(a.dir_id)?.sort ?? 0
    const sb = dirSortMap.value.get(b.dir_id)?.sort ?? 0
    return sa - sb || a.dir_id - b.dir_id
  })
  return groups
})

const rows = ref<Row[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const published = ref<string>('')
const selection = ref<Row[]>([])
const dirName = ref('')

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
  if (dirID.value) {
    const find = (list: TreeNode[]): string => {
      for (const n of list) {
        if (n.type === 'dir' && n.id === dirID.value) return n.label
        const sub = find(n.children || [])
        if (sub) return sub
      }
      return ''
    }
    dirName.value = find(treeData.value)
  }
}

function mapPost(p: Post): Row {
  return { type: 'post', ...p }
}
function mapPage(p: Page): Row {
  return {
    type: 'page', id: p.id, dir_id: p.dir_id, title: p.title, slug: p.slug,
    tags: p.tags || [], is_published: p.is_published, sort: p.sort,
    updated_at: p.updated_at, dir_name: p.dir_name, dir_path: p.dir_path,
  }
}

async function load() {
  loading.value = true
  try {
    if (mixed.value) {
      const posts: Row[] = []
      const pages: Row[] = []
      let p = 1
      for (;;) {
        const res = await postApi.list({ dir_id: dirID.value, page: p, page_size: 100 })
        posts.push(...res.list.map(mapPost))
        if (posts.length >= res.total || !res.list.length) break
        p++
      }
      p = 1
      for (;;) {
        const res = await pageApi.list({ dir_id: dirID.value, page: p, page_size: 100 })
        pages.push(...res.list.map(mapPage))
        if (pages.length >= res.total || !res.list.length) break
        p++
      }
      rows.value = [...posts, ...pages].sort((a, b) => a.sort - b.sort || a.id - b.id)
      total.value = rows.value.length
    } else {
      const res = await postApi.list({
        page: page.value,
        page_size: pageSize.value,
        keyword: keyword.value || undefined,
        is_published: published.value === '' ? undefined : published.value,
      })
      rows.value = res.list.map(mapPost)
      total.value = res.total
    }
  } finally {
    loading.value = false
  }
}

// 非 mixed 模式：rows 为当前页数据；mixed 模式：rows 为全量数据（不分页）
const pagedRows = computed(() => rows.value)

function search() {
  page.value = 1
  load()
}

function onSelectionChange(rows: Row[]) {
  selection.value = rows
}

function onPageChange() {
  if (!mixed.value) load()
}

function onSizeChange() {
  page.value = 1
  if (!mixed.value) load()
}

function goEdit(row: Row) {
  router.push(row.type === 'post' ? `/posts/edit/${row.id}` : `/pages/edit/${row.id}`)
}

function goDir(id: number) {
  router.push({ path: '/posts', query: { dir_id: String(id) } })
}

async function removeRow(row: Row) {
  await ElMessageBox.confirm(`确定删除「${row.title}」吗？`, '删除确认', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  if (row.type === 'post') await postApi.remove(row.id)
  else await pageApi.remove(row.id)
  ElMessage.success('已删除')
  load()
  emit('tree-refresh')
}

async function exportRow(row: Row) {
  if (row.type === 'post') await postApi.exportOne(row.id, `${row.slug}.md`)
  else await pageApi.exportOne(row.id, `${row.slug}.md`)
}

async function batchExport() {
  if (!selection.value.length) {
    ElMessage.warning('请先勾选内容')
    return
  }
  const posts = selection.value.filter((r) => r.type === 'post').map((r) => r.id)
  const pages = selection.value.filter((r) => r.type === 'page').map((r) => r.id)
  if (posts.length) await postApi.batchExport(posts)
  if (pages.length) await pageApi.batchExport(pages)
}

function openImport() {
  importDir.value = dirID.value
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
    const res = await postApi.importZip(importFile.value, importDir.value)
    importDialog.value = false
    ElMessage.success(`导入完成：成功 ${res.imported}，跳过 ${res.skipped}`)
    if (res.errors.length) {
      ElMessage.warning(res.errors.slice(0, 3).map((e) => `${e.file}: ${e.reason}`).join('；'))
    }
    emit('tree-refresh')
    load()
    onTreeChanged()
  } finally {
    importing.value = false
  }
  
}

function newPost() {
  router.push({ path: '/posts/edit/0', query: mixed.value ? { dir_id: String(dirID.value) } : {} })
}
function newPage() {
  router.push({ path: '/pages/edit/0', query: mixed.value ? { dir_id: String(dirID.value) } : {} })
}

function onTreeChanged() {
  loadTree()
  load()
}

watch(() => route.query.dir_id, () => {
  page.value = 1
  loadTree()
  load()
})

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
        <h2 class="page-title">{{ mixed ? `分类：${dirName || '未知'}` : '文章列表' }}</h2>
        <p class="page-subtitle">{{ mixed ? '该分类下的文章与页面（混合视图）' : '全部文章' }}</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="newPost">新建文章</el-button>
        <el-button @click="newPage">新建页面</el-button>
        <el-button @click="openImport">导入文章</el-button>
        <el-button :disabled="!selection.length" @click="batchExport">批量导出</el-button>
      </div>
    </div>

    <div class="card" style="margin-bottom: 14px" v-if="!mixed">
      <div class="filter-row">
        <el-input v-model="keyword" placeholder="搜索标题/内容" clearable @keyup.enter="search" />
        <el-select v-model="published" placeholder="发布状态" clearable @change="search">
          <el-option label="已发布" value="1" />
          <el-option label="未发布" value="0" />
        </el-select>
        <el-button type="primary" plain @click="search">查询</el-button>
      </div>
    </div>

    <div class="card">
      <!-- 分组模式：一级目录聚合多个子目录时按子分类分组 -->
      <template v-if="groupMode">
        <div v-for="g in groupedRows" :key="g.dir_id" class="group-section">
          <div class="group-header">
            <span class="group-title">{{ g.dir_name }}</span>
            <el-tag size="small" type="info">{{ g.rows.length }} 项</el-tag>
            <el-link type="primary" class="group-more" @click="goDir(g.dir_id)">查看 →</el-link>
          </div>
          <el-table :data="g.rows" size="small" :show-header="false">
            <el-table-column width="44" />
            <el-table-column min-width="220">
              <template #default="{ row }">
                <el-link type="primary" @click="goEdit(row as Row)">{{ (row as Row).title }}</el-link>
                <el-tag size="small" :type="(row as Row).type === 'post' ? 'primary' : 'info'" style="margin-left: 6px">
                  {{ (row as Row).type === 'post' ? '文章' : '页面' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="slug" min-width="140">
              <template #default="{ row }">{{ (row as Row).slug }}</template>
            </el-table-column>
            <el-table-column width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="(row as Row).is_published ? 'success' : 'info'">
                  {{ (row as Row).is_published ? '已发布' : '草稿' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" width="170">
              <template #default="{ row }">{{ formatDate((row as Row).updated_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="180">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="goEdit(row as Row)">编辑</el-button>
                <el-button size="small" text @click="exportRow(row as Row)">导出</el-button>
                <el-button size="small" text type="danger" @click="removeRow(row as Row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </template>

      <!-- 扁平模式：无子目录或非 mixed 视图 -->
      <template v-else>
        <el-table
          v-loading="loading"
          :data="pagedRows"
          size="small"
          @selection-change="onSelectionChange"
        >
          <el-table-column type="selection" width="44" />
          <el-table-column label="标题" min-width="220">
            <template #default="{ row }">
              <el-link type="primary" @click="goEdit(row as Row)">{{ row.title }}</el-link>
              <el-tag v-if="mixed" size="small" :type="row.type === 'post' ? 'primary' : 'info'" style="margin-left: 6px">
                {{ row.type === 'post' ? '文章' : '页面' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column v-if="mixed" label="所属分类" min-width="180">
            <template #default="{ row }">
              <span v-if="row.dir_path" class="muted">{{ row.dir_path }}</span>
              <el-tag v-else-if="row.dir_name" size="small">{{ row.dir_name }}</el-tag>
              <span v-else class="muted">-</span>
            </template>
          </el-table-column>
          <el-table-column prop="slug" label="slug" min-width="140" />
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
              <el-button size="small" text type="primary" @click="goEdit(row as Row)">编辑</el-button>
              <el-button size="small" text @click="exportRow(row as Row)">导出</el-button>
              <el-button size="small" text type="danger" @click="removeRow(row as Row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-pagination
          v-if="!mixed"
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          style="margin-top: 14px; justify-content: flex-end"
          @current-change="onPageChange"
          @size-change="onSizeChange"
        />
      </template>
    </div>

    <el-dialog v-model="importDialog" title="导入文章" width="460px">
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
      <div class="muted">支持单篇 Markdown 或包含 manifest.json 的 zip 包；导入内容统一归入所选分类。</div>
      <template #footer>
        <el-button @click="importDialog = false">取消</el-button>
        <el-button type="primary" :loading="importing" @click="doImport">开始导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>
