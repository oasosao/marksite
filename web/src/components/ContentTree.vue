<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Document, FolderOpened, Tickets } from '@element-plus/icons-vue'
import { treeApi } from '../api'
import { postApi, pageApi } from '../api'
import type { TreeNode } from '../types'
import { on, off, emit } from '../utils/bus'
import { flattenDirs } from '../utils/dirTree'
import { slugify } from '../utils/format'

const router = useRouter()
const treeRef = ref()
// 整棵树只在刷新时整体替换，从不原地修改，用 shallowRef 避免 Vue 深度代理每个节点
const treeData = shallowRef<TreeNode[]>([])
const filterText = ref('')
const expandAll = ref(false)
const treeKey = ref(0)
const loading = ref(false)
// 用户真实的展开状态放在普通 Set 里，避免每次展开/收起都触发响应式更新
const expandedSet = new Set<string>()
// 仅作为 el-tree 首次挂载 / 整树重建时的展开依据，只在这些时机同步
const expandedKeys = shallowRef<string[]>([])
let lastSnapshot = ''
const treeProps = { label: 'label', children: 'children' }

const menu = ref<{ visible: boolean; x: number; y: number; node: TreeNode | null; canUp: boolean; canDown: boolean }>({
  visible: false, x: 0, y: 0, node: null, canUp: false, canDown: false,
})

const moveDialog = ref<{ visible: boolean; node: TreeNode | null; target: number }>({
  visible: false, node: null, target: 0,
})
const dirOptions = computed(() => [{ value: 0, label: '根分类' }, ...flattenDirs(treeData.value).map((d) => ({
  value: d.value,
  label: `${'\u00a0\u00a0'.repeat(d.depth)}${d.label}`,
}))])

const deleteDialog = ref<{ visible: boolean; node: TreeNode | null; mode: 'move' | 'cascade' }>({
  visible: false, node: null, mode: 'move',
})

const createDirDialog = ref<{
  visible: boolean
  parent_id: number
  name: string
  slug: string
  is_page: number
  slugTouched: boolean
}>({ visible: false, parent_id: 0, name: '', slug: '', is_page: 0, slugTouched: false })

function getCurrentDirID(): number {
  const node = treeRef.value?.getCurrentNode() as TreeNode | null
  if (node?.type === 'dir') return node.id
  if (node) return node.parent_id || 0
  return 0
}

function newPostFromTree() {
  router.push({ path: '/posts/edit/0', query: { dir_id: String(getCurrentDirID()) } })
}
function newPageFromTree() {
  router.push({ path: '/pages/edit/0', query: { dir_id: String(getCurrentDirID()) } })
}

async function load() {
  // 只有首次加载才显示 loading 遮罩，后续静默刷新，避免遮罩反复闪动
  loading.value = treeData.value.length === 0
  try {
    const data = await treeApi.get()
    // 数据没变化就不替换 treeData，避免 el-tree 整棵树重建
    const snapshot = JSON.stringify(data)
    if (snapshot === lastSnapshot) return
    const firstLoad = lastSnapshot === ''
    lastSnapshot = snapshot
    if (firstLoad) {
      expandedSet.clear()
      if (expandAll.value) collectDirKeys(data).forEach((k) => expandedSet.add(k))
    } else {
      // 删除已不存在的分类，避免残留 key
      const alive = new Set(collectDirKeys(data))
      for (const k of expandedSet) if (!alive.has(k)) expandedSet.delete(k)
    }
    // 先同步展开状态、再替换数据：重建 Node 时只会展开用户当前真正展开的目录
    syncExpandedProp()
    treeData.value = data
    // 树重建后过滤结果会丢失，若仍有过滤词则重新应用
    if (filterText.value) nextTick(() => onFilter())
  } finally {
    loading.value = false
  }
}

// 只在展开集合真的变化时才更新 default-expanded-keys（该 prop 每次变化都会让 el-tree 遍历展开）。
// Set 的迭代顺序随增删变化，先排序再比较，避免顺序差异被误判为内容变化
function syncExpandedProp() {
  const next = [...expandedSet].sort()
  const cur = expandedKeys.value
  if (next.length === cur.length && next.every((k, i) => cur[i] === k)) return
  expandedKeys.value = next
}

function filterNode(value: string, data: TreeNode) {
  if (!value) return true
  return data.label.toLowerCase().includes(value.toLowerCase())
}

function onFilter() {
  treeRef.value?.filter(filterText.value)
}

// 过滤会遍历整棵树并逐批 nextTick，输入时做防抖，避免每敲一个字都全树扫描
let filterTimer: number | null = null
function onFilterInput() {
  if (filterTimer !== null) window.clearTimeout(filterTimer)
  filterTimer = window.setTimeout(() => {
    filterTimer = null
    onFilter()
  }, 200)
}

function toggleExpand() {
  expandAll.value = !expandAll.value
  expandedSet.clear()
  if (expandAll.value) collectDirKeys(treeData.value).forEach((k) => expandedSet.add(k))
  syncExpandedProp()
  // 只有「全部展开/收起」才重建树，日常数据刷新不会动这里
  treeKey.value++
  nextTick(() => onFilter())
}

// 收集当前树里所有可见分类的 key（用于首次加载和「展开全部」）
function collectDirKeys(list: TreeNode[]): string[] {
  const keys: string[] = []
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      if (n.type === 'dir' && n.menu_visible == 1) {
        keys.push(n.key)
        walk(n.children || [])
      }
    }
  }
  walk(list)
  return keys
}

// 记录用户手动展开/收起的分类，刷新数据后按这个状态恢复，而不是重新全部展开。
// 只改普通 Set，不触发组件重新渲染
function onNodeExpand(data: TreeNode) {
  if (data.type === 'dir') expandedSet.add(data.key)
}

function onNodeCollapse(data: TreeNode) {
  if (data.type === 'dir') expandedSet.delete(data.key)
}

function nodeLabelClass(node: TreeNode) {
  return node.type
}

function handleClick(data: TreeNode) {
  // 长按唤出菜单后，紧接着触发的那次 click 不应再跳转
  if (suppressClick) {
    suppressClick = false
    return
  }
  if (data.type === 'dir') {
    router.push({ path: '/posts', query: { dir_id: String(data.id) } })
  } else if (data.type === 'post') {
    router.push(`/posts/edit/${data.id}`)
  } else {
    router.push(`/pages/edit/${data.id}`)
  }
}

// 触摸屏没有右键，改用长按或节点右侧的「⋯」按钮唤出同一套操作菜单
let longPressTimer: number | null = null
let suppressClick = false
let menuOpenedAt = 0

function openMenuAtPoint(x: number, y: number, data: TreeNode) {
  // 限制在视口内，避免小屏溢出
  const menuW = 200
  const menuH = 360
  const siblings = siblingList(data)
  const idx = siblings ? siblings.findIndex((n) => n.key === data.key) : -1
  menuOpenedAt = Date.now()
  menu.value = {
    visible: true,
    x: Math.max(8, Math.min(x, window.innerWidth - menuW - 8)),
    y: Math.max(8, Math.min(y, window.innerHeight - menuH - 8)),
    node: data,
    canUp: idx > 0,
    canDown: idx >= 0 && !!siblings && idx < siblings.length - 1,
  }
}

// 找到节点所在的兄弟数组，用于上移/下移（触屏没有拖拽，用它替代同级排序）
function siblingList(node: TreeNode): TreeNode[] | null {
  const walk = (list: TreeNode[]): TreeNode[] | null => {
    for (const n of list) {
      if (n.key === node.key) return list
      if (n.children?.length) {
        const found = walk(n.children)
        if (found) return found
      }
    }
    return null
  }
  return walk(treeData.value)
}

async function moveSibling(delta: number) {
  const node = menu.value.node
  closeMenu()
  if (!node) return
  const siblings = siblingList(node)
  if (!siblings) return
  const idx = siblings.findIndex((n) => n.key === node.key)
  const target = idx + delta
  if (idx < 0 || target < 0 || target >= siblings.length) return
  const next = siblings.slice()
  const [moved] = next.splice(idx, 1)
  next.splice(target, 0, moved)
  await treeApi.sort(
    node.parent_id || 0,
    next.map((n, i) => ({ type: n.type, id: n.id, sort: i })),
  )
  ElMessage.success('已调整顺序')
  await refresh()
}

function menuMoveUp() {
  moveSibling(-1)
}

function menuMoveDown() {
  moveSibling(1)
}

function openMenu(event: MouseEvent, data: TreeNode) {
  event.preventDefault()
  openMenuAtPoint(event.clientX, event.clientY, data)
}

function openMenuFromButton(event: MouseEvent, data: TreeNode) {
  event.stopPropagation()
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  openMenuAtPoint(rect.right - 8, rect.bottom + 4, data)
}

function onNodeTouchStart(event: TouchEvent, data: TreeNode) {
  const touch = event.touches[0]
  if (!touch) return
  cancelLongPress()
  longPressTimer = window.setTimeout(() => {
    longPressTimer = null
    suppressClick = true
    openMenuAtPoint(touch.clientX, touch.clientY, data)
    window.setTimeout(() => { suppressClick = false }, 400)
  }, 450)
}

function cancelLongPress() {
  if (longPressTimer !== null) {
    window.clearTimeout(longPressTimer)
    longPressTimer = null
  }
}

function closeMenu() {
  menu.value.visible = false
}

function createDir(parentID: number) {
  createDirDialog.value = {
    visible: true, parent_id: parentID, name: '', slug: '', is_page: 0, slugTouched: false,
  }
}

// 树头部「＋」下拉：新建时带上当前选中节点所在的分类
function onCreateCommand(command: string) {
  if (command === 'dir') createDir(getCurrentDirID())
  else if (command === 'post') newPostFromTree()
  else newPageFromTree()
}

function onDirNameInput(value: string) {
  if (createDirDialog.value.slugTouched) return
  const slug = slugify(value)
  if (slug) {
    createDirDialog.value.slug = slug
  } else {
    createDirDialog.value.slug = ''
  }
}

async function confirmCreateDir() {
  const d = createDirDialog.value
  const name = d.name.trim()
  const slug = d.slug.trim()
  if (!name) {
    ElMessage.warning('名称不能为空')
    return
  }
  if (slug && !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
    ElMessage.warning('slug 仅允许小写字母、数字与中划线；留空则后端自动生成')
    return
  }
  await treeApi.createDir({
    parent_id: d.parent_id,
    name,
    slug: slug || undefined,
    is_page: d.is_page,
    is_published: 1,
    menu_visible: 1,
  })
  d.visible = false
  ElMessage.success(d.is_page === 1 ? '独立页面已创建' : '分类已创建')
  await refresh()
}

function editDirNode(node: TreeNode) {
  router.push(`/dirs/edit/${node.id}`)
}

async function renameDir(node: TreeNode) {
  const { value } = await ElMessageBox.prompt('请输入新的分类名称', '重命名分类', {
    confirmButtonText: '保存',
    cancelButtonText: '取消',
    inputValue: node.label,
    inputPattern: /\S+/,
    inputErrorMessage: '分类名称不能为空',
  })
  await treeApi.updateDir(node.id, { name: value.trim() })
  ElMessage.success('已重命名')
  await refresh()
}

function openMove(node: TreeNode) {
  moveDialog.value = { visible: true, node, target: node.parent_id || 0 }
}

function menuCreateDir() {
  const node = menu.value.node
  closeMenu()
  if (node) createDir(node.id)
}

function menuEditDir() {
  const node = menu.value.node
  closeMenu()
  if (node) editDirNode(node)
}

function menuNewPost() {
  const node = menu.value.node
  closeMenu()
  if (node) router.push({ path: '/posts/edit/0', query: { dir_id: String(node.id) } })
}

function menuNewPage() {
  const node = menu.value.node
  closeMenu()
  if (node) router.push({ path: '/pages/edit/0', query: { dir_id: String(node.id) } })
}

function menuRename() {
  const node = menu.value.node
  closeMenu()
  if (node) renameDir(node)
}

function menuMove() {
  const node = menu.value.node
  closeMenu()
  if (node) openMove(node)
}

function menuDelete() {
  const node = menu.value.node
  closeMenu()
  if (node) openDelete(node)
}

function menuEdit() {
  const node = menu.value.node
  closeMenu()
  if (node) handleClick(node)
}

async function confirmMove() {
  const node = moveDialog.value.node
  if (!node) return
  const target = moveDialog.value.target || 0
  if (node.type === 'dir') {
    await treeApi.updateDir(node.id, { parent_id: target })
  } else if (node.type === 'post') {
    await treeApi.movePost(node.id, target)
  } else {
    await treeApi.movePage(node.id, target)
  }
  moveDialog.value.visible = false
  ElMessage.success('移动成功')
  await refresh()
}

function openDelete(node: TreeNode) {
  if (node.type === 'dir') {
    deleteDialog.value = { visible: true, node, mode: 'move' }
    return
  }
  ElMessageBox.confirm(`确定删除「${node.label}」吗？删除后不可恢复。`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  })
    .then(async () => {
      if (node.type === 'post') await postApi.remove(node.id)
      else await pageApi.remove(node.id)
      ElMessage.success('已删除')
      await refresh()
    })
    .catch(() => undefined)
}

async function confirmDeleteDir() {
  const node = deleteDialog.value.node
  if (!node) return
  const res = await treeApi.deleteDir(node.id, deleteDialog.value.mode)
  deleteDialog.value.visible = false
  if (deleteDialog.value.mode === 'move') {
    ElMessage.success(`已删除分类，移动子分类 ${res.moved_dirs} 个、文章 ${res.moved_posts} 篇、页面 ${res.moved_pages} 个`)
  } else {
    ElMessage.success(`已级联删除分类 ${res.deleted_dirs} 个、文章 ${res.deleted_posts} 篇、页面 ${res.deleted_pages} 个`)
  }
  await refresh()
}

async function refresh() {
  await load()
  emit('content-refresh')
}

function allowDrop(dragging: any, drop: any, type: 'before' | 'after' | 'inner') {
  const source = dragging.data as TreeNode
  const target = drop.data as TreeNode
  if (type === 'inner') {
    if (target.type !== 'dir') return false
    if (target.id === source.id) return false
    if (source.type === 'dir') {
      let cur = drop.parent
      while (cur) {
        if (cur.data?.type === 'dir' && cur.data.id === source.id) return false
        cur = cur.parent
      }
    }
    return true
  }
  const sourceParent = dragging.parent ? dragging.parent.data.key : 'root'
  const targetParent = drop.parent ? drop.parent.data.key : 'root'
  return sourceParent === targetParent
}

async function handleDrop(dragging: any, drop: any, type: 'before' | 'after' | 'inner') {
  const source = dragging.data as TreeNode
  const target = drop.data as TreeNode
  try {
    if (type === 'inner') {
      if (source.type === 'dir') await treeApi.updateDir(source.id, { parent_id: target.id })
      else if (source.type === 'post') await treeApi.movePost(source.id, target.id)
      else await treeApi.movePage(source.id, target.id)
    } else {
      const parentNode = drop.parent
      const parentID = parentNode ? (parentNode.data as TreeNode).id : 0
      const siblings: TreeNode[] = (parentNode ? parentNode.childNodes : treeRef.value.root.childNodes).map(
        (n: any) => n.data,
      )
      await treeApi.sort(
        parentID,
        siblings.map((n, idx) => ({ type: n.type, id: n.id, sort: idx })),
      )
    }
    ElMessage.success('操作成功')
  } catch {
    // 错误提示已由 http 层处理
  } finally {
    await refresh()
  }
}

function handleBodyClick() {
  // 长按唤出菜单后浏览器还会补发一次 click，忽略这次以免菜单瞬间被关掉
  if (Date.now() - menuOpenedAt < 400) return
  closeMenu()
}

onMounted(() => {
  load()
  on('tree-refresh', refresh)
  document.addEventListener('click', handleBodyClick)
})

onBeforeUnmount(() => {
  off('tree-refresh', refresh)
  document.removeEventListener('click', handleBodyClick)
  cancelLongPress()
  if (filterTimer !== null) {
    window.clearTimeout(filterTimer)
    filterTimer = null
  }
})

</script>

<template>
  <div class="tree-wrap">
    <div class="tree-tools">
      <el-input v-model="filterText" placeholder="过滤内容" clearable @input="onFilterInput" />
      <el-dropdown trigger="click" @command="onCreateCommand">
        <el-button type="primary" plain title="新建">＋</el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="dir">新建分类</el-dropdown-item>
            <el-dropdown-item command="post">新建文章</el-dropdown-item>
            <el-dropdown-item command="page">新建页面</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <el-button default @click="toggleExpand">{{ expandAll ? '收起' : '展开' }}</el-button>
    </div>
    <div class="tree-scroll" v-loading="loading">
      <el-tree ref="treeRef" :key="treeKey" 
        :data="treeData" 
        node-key="key"
        :props="treeProps"
        :default-expanded-keys="expandedKeys"
        :auto-expand-parent="false"
        :filter-node-method="filterNode as any" 
        :expand-on-click-node='false' 
        highlight-current 
        draggable
        :allow-drop='allowDrop as any' 
        @node-click='handleClick as any' 
        @node-expand='onNodeExpand as any'
        @node-collapse='onNodeCollapse as any'
        @node-drop='handleDrop'
        @node-contextmenu='openMenu as any'>

        <template #default='{ data }'>
          <span class='node-row' 
            :title='data.label' 
            @touchstart.passive='onNodeTouchStart($event, data)'
            @touchmove.passive='cancelLongPress' 
            @touchend='cancelLongPress' 
            @touchcancel='cancelLongPress'>

            <span class='node-label' :class='nodeLabelClass(data)'>
              <template v-if='data.type === "dir"'>
                <el-icon class="node-icon">
                  <FolderOpened v-if="!data.is_page" />
                  <Document v-else />
                </el-icon>
                <span class="node-text">{{ data.label }}</span>
                <span v-if="data.slug" class="slug-badge">/{{ data.slug }}</span>
                <span v-if="!data.is_published" class="dot-off" title="未发布"></span>
              </template>
              <template v-else-if="data.type === 'post'">
                <el-icon class="node-icon">
                  <Document />
                </el-icon>
                <span class="node-text">{{ data.label }}</span>
                <span v-if="data.is_published" class="dot" title="已发布"></span>
                <span v-else class="dot-off" title="未发布"></span>
              </template>
              <template v-else>
                <el-icon class="node-icon">
                  <Tickets />
                </el-icon>
                <span class="node-text">{{ data.label }}</span>
                <span v-if="data.is_published" class="dot" title="已发布"></span>
                <span v-else class="dot-off" title="未发布"></span>
              </template>
            </span>
            <button class="node-more" type="button" title="更多操作" aria-label="更多操作" @click="openMenuFromButton($event, data)">⋯</button>
          </span>
        </template>
      </el-tree>
    </div>

    <div v-if="menu.visible && menu.node" class="context-menu" :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
      @click.stop>
      <!-- 菜单顶部显示完整名称，方便长标题被截断时确认对象 -->
      <div class="menu-title" :title="menu.node.label">{{ menu.node.label }}</div>
      <template v-if="menu.node.type === 'dir'">
        <div @click="menuCreateDir">新建子分类</div>
        <template v-if="!menu.node.is_page">
          <div @click="menuNewPost">新建文章</div>
          <div @click="menuNewPage">新建页面</div>
        </template>
        <div @click="menuEditDir">编辑{{ menu.node.is_page ? '页面' : '分类' }}</div>
        <div @click="menuRename">重命名</div>
        <div :class="{ disabled: !menu.canUp }" @click="menu.canUp && menuMoveUp()">上移</div>
        <div :class="{ disabled: !menu.canDown }" @click="menu.canDown && menuMoveDown()">下移</div>
        <div @click="menuMove">移动到…</div>
        <div class="danger" @click="menuDelete">删除分类</div>
      </template>
      <template v-else>
        <div @click="menuEdit">编辑</div>
        <div :class="{ disabled: !menu.canUp }" @click="menu.canUp && menuMoveUp()">上移</div>
        <div :class="{ disabled: !menu.canDown }" @click="menu.canDown && menuMoveDown()">下移</div>
        <div @click="menuMove">移动到…</div>
        <div class="danger" @click="menuDelete">删除</div>
      </template>
    </div>

    <el-dialog v-model="createDirDialog.visible" title="新建分类 / 独立页面" width="460px">
      <el-form label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="createDirDialog.name" placeholder="例如：技术博客" @input="onDirNameInput" />
        </el-form-item>
        <el-form-item label="slug" required>
          <el-input v-model="createDirDialog.slug" placeholder="小写字母、数字、中划线"
            @input="createDirDialog.slugTouched = true" />
          <div class="muted" style="font-size: 12px; margin-top: 4px">
            {{ createDirDialog.is_page ? `生成路径：/${createDirDialog.slug || 'slug'}.html`
              : `生成路径：/${createDirDialog.slug || 'slug'}/index.html` }}
          </div>
        </el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="createDirDialog.is_page">
            <el-radio :value="0">分类（可添加文章）</el-radio>
            <el-radio :value="1">独立页面</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDirDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="confirmCreateDir">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="moveDialog.visible" title="移动到" width="420px">
      <el-tree-select v-model="moveDialog.target" :data="dirOptions" check-strictly :render-after-expand="false"
        placeholder="选择目标分类" style="width: 100%" />
      <template #footer>
        <el-button @click="moveDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="confirmMove">确认移动</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="deleteDialog.visible" title="删除分类" width="460px">
      <p class="muted">请选择删除方式：</p>
      <el-radio-group v-model="deleteDialog.mode">
        <el-radio value="move">保留内容并移动到上级分类</el-radio>
        <el-radio value="cascade">级联删除全部子分类与内容（不可恢复）</el-radio>
      </el-radio-group>
      <template #footer>
        <el-button @click="deleteDialog.visible = false">取消</el-button>
        <el-button type="danger" @click="confirmDeleteDir">确认删除</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.tree-wrap {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.tree-tools {
  display: flex;
  gap: 6px;
  padding: 8px;
  align-items: center;
}

.tree-tools :deep(.el-input) {
  flex: 1 1 auto;
  min-width: 0;
}

.tree-scroll {
  flex: 1;
  overflow: auto;
  min-height: 0;
  padding: 0 6px 8px;
  -webkit-overflow-scrolling: touch;
  overscroll-behavior: contain;
}

:deep(.el-tree-node__content) {
  height: 32px;
  border-radius: 6px;
}

:deep(.el-tree-node__label) {
  flex: 1 1 auto;
  min-width: 0;
}

.node-row {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  min-width: 0;
  /* 触屏长按时不选中文字、不弹出系统callout */
  -webkit-user-select: none;
  user-select: none;
  -webkit-touch-callout: none;
}

.node-label {
  flex: 1 1 auto;
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
  white-space: nowrap;
}

.node-icon {
  flex: 0 0 auto;
  color: #9ca3af;
  font-size: 14px;
}

/* 标题单独成项，过长时显示省略号而不是硬裁切 */
.node-text {
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 节点右侧「⋯」：常驻低对比度，悬停/触屏放大，替代不可发现的右键 */
.node-more {
  flex: 0 0 auto;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #6b7280;
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  opacity: 0.35;
  transition: opacity 0.15s ease, background 0.15s ease;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.node-more:hover {
  opacity: 1;
  background: #eef3fe;
  color: var(--el-color-primary);
}

:deep(.el-tree-node__content:hover) .node-more {
  opacity: 1;
}

.slug-badge {
  flex: 0 0 auto;
  font-size: 11px;
  color: #8b93a3;
  background: #f2f4f8;
  border-radius: 4px;
  padding: 0 4px;
  max-width: 90px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dot-off {
  flex: 0 0 auto;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #c0c4cc;
  display: inline-block;
}

/* 菜单顶部名称与置灰项 */
.menu-title {
  padding: 6px 12px 8px;
  margin-bottom: 2px;
  border-bottom: 1px solid #eef0f4;
  font-weight: 600;
  color: #111827;
  cursor: default;
  max-width: 220px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.menu-title:hover {
  background: transparent !important;
}

.context-menu .disabled {
  color: #c0c4cc;
  cursor: default;
}

.context-menu .disabled:hover {
  background: transparent;
}

/* —— 触屏 / 小屏：放大点击区域，操作按钮常驻 —— */
@media (max-width: 1024px),
(pointer: coarse) {
  :deep(.el-tree-node__content) {
    height: 44px;
  }

  .node-label {
    font-size: 14px;
  }

  .node-more {
    opacity: 1;
    width: 34px;
    height: 34px;
    font-size: 18px;
  }

  .tree-tools :deep(.el-input__wrapper) {
    min-height: 38px;
  }

  .tree-tools :deep(.el-button) {
    height: 38px;
    padding: 0 12px;
  }

  .slug-badge {
    display: none;
  }
}

.node-label.page{
  padding:0;
}
</style>
