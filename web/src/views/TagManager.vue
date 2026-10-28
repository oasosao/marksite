<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tagApi } from '../api'
import type { TagCount } from '../types'
import { formatDate, slugify } from '../utils/format'

const list = ref<TagCount[]>([])
const loading = ref(false)
const keyword = ref('')

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return list.value
  return list.value.filter((t) => t.name.toLowerCase().includes(kw) || t.slug.includes(kw))
})

const dialog = ref<{
  visible: boolean
  id: number
  name: string
  slug: string
  sort: number
  slugTouched: boolean
}>({ visible: false, id: 0, name: '', slug: '', sort: 0, slugTouched: false })

async function load() {
  loading.value = true
  try {
    list.value = await tagApi.list()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  dialog.value = { visible: true, id: 0, name: '', slug: '', sort: 0, slugTouched: false }
}

function openEdit(tag: TagCount) {
  dialog.value = {
    visible: true, id: tag.id, name: tag.name, slug: tag.slug, sort: tag.sort, slugTouched: true,
  }
}

function onNameInput(value: string) {
  if (dialog.value.slugTouched) return
  dialog.value.slug = slugify(value)
}

async function save() {
  const d = dialog.value
  const name = d.name.trim()
  const slug = d.slug.trim()
  if (!name) {
    ElMessage.warning('标签名不能为空')
    return
  }
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
    ElMessage.warning('slug 仅允许小写字母、数字与中划线（中文标签请手动填写）')
    return
  }
  if (d.id) {
    await tagApi.update(d.id, { name, slug, sort: d.sort })
    ElMessage.success('已保存')
  } else {
    await tagApi.create({ name, slug, sort: d.sort })
    ElMessage.success('标签已创建')
  }
  d.visible = false
  await load()
}

async function remove(tag: TagCount) {
  await ElMessageBox.confirm(
    `确定删除标签「${tag.name}」吗？已关联的文章/页面会解除该标签。`,
    '删除确认',
    { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
  )
  await tagApi.remove(tag.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="page-header">
      <div>
        <h2 class="page-title">标签管理</h2>
        <p class="page-subtitle">标签拥有独立 slug，生成到 /tags/{slug}.html</p>
      </div>
      <div class="page-actions">
        <el-input v-model="keyword" size="default" placeholder="搜索标签" clearable />
        <el-button type="primary" @click="openCreate">新建标签</el-button>
      </div>
    </div>

    <div class="card">
      <el-table :data="filtered" size="small">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="标签名" min-width="160" />
        <el-table-column prop="slug" label="slug" min-width="160" />
        <el-table-column prop="use_count" label="引用数" width="90" />
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="130">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row as TagCount)">编辑</el-button>
            <el-button size="small" text type="danger" @click="remove(row as TagCount)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialog.visible" :title="dialog.id ? '编辑标签' : '新建标签'" width="440px">
      <el-form label-width="80px">
        <el-form-item label="标签名" required>
          <el-input v-model="dialog.name" placeholder="例如：Go语言" @input="onNameInput" />
        </el-form-item>
        <el-form-item label="slug" required>
          <el-input
            v-model="dialog.slug"
            placeholder="小写字母、数字、中划线"
            @input="dialog.slugTouched = true"
          />
          <div class="muted" style="font-size: 12px; margin-top: 4px">
            生成路径：/tags/{{ dialog.slug || 'slug' }}.html
          </div>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="dialog.sort" :min="0" :max="99999" controls-position="right" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
