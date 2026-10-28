<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Brush, DataLine, Files, House, List, Monitor, Picture, PriceTag, RefreshRight, Search, Setting, View } from '@element-plus/icons-vue'
import ContentTree from '../components/ContentTree.vue'
import TaskProgressDialog from '../components/TaskProgressDialog.vue'
import { configApi, generateApi } from '../api'
import { off, on } from '../utils/bus'
import { desktopShowSites, isDesktop } from '../utils/desktop'
import type { Task } from '../types'

const route = useRoute()
const siteName = ref('MarkSite')

const genDialog = ref(false)
const taskId = ref(0)


async function runGenerate(mode: 'full' | 'increment') {
  if (mode === 'full') {
    await ElMessageBox.confirm(
      '全量生成会清空 output 目录并重建全部静态文件，确认继续？',
      '全量生成确认',
      { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' },
    )
  }
  const res = mode === 'full' ? await generateApi.full() : await generateApi.increment()
  taskId.value = res.task_id
  genDialog.value = true
}

function onGenDone(task: Task) {
  if (task.status === 'success') ElMessage.success('生成完成')
  else if (task.status === 'failed') ElMessage.error(task.error || '生成失败')
}

// 桌面端：退出当前站点，回到站点管理页重新选择或新建站点
async function manageSites() {
  try {
    await ElMessageBox.confirm(
      '将关闭当前站点并返回站点管理页，未保存的编辑内容会丢失。',
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

const isNarrow = ref(true)  

// 桌面端侧栏可收起为图标条，把空间让给编辑区（宽屏笔记本常用）
const sidebarCollapsed = ref(false)

function toggleSidebar() {
  isNarrow.value = !isNarrow.value
  sidebarCollapsed.value = !sidebarCollapsed.value
}


const fixedEntries = [
  { path: '/dashboard', icon: DataLine, label: '仪表盘' },
  { path: '/posts', icon: List, label: '文章列表' },
  { path: '/assets', icon: Picture, label: '资源管理' },
  { path: '/preview', icon: View, label: '预览站点' },
  { path: '/tags', icon: PriceTag, label: '标签管理' },
  { path: '/themes', icon: Brush, label: '主题管理' },
  { path: '/fts-index', icon: Search, label: '搜索索引管理' },
  { path: '/backup', icon: Files, label: '备份恢复' },
  { path: '/setting/app', icon: Setting, label: '程序设置' },
  { path: '/setting/site', icon: Monitor, label: '网站生成设置' },

]

const activePath = ref('')

function syncActive() {
  if (route.path.startsWith('/posts') || route.path.startsWith('/pages')) {
    activePath.value = ''
  } else {
    activePath.value = route.path
  }
}

async function loadSite() {
  try {
    const cfg = await configApi.getSite()
    siteName.value = cfg.site_name || 'MarkSite'
  } catch {
    // 站点信息加载失败时保持默认名称即可
  }
}

watch(() => route.path, syncActive, { immediate: true })

onMounted(() => {
  loadSite()
  on('site-refresh', loadSite)
})
onBeforeUnmount(() => {
  off('site-refresh', loadSite)
})
</script>

<template>

  <div class="layout" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <!-- 小屏抽屉蒙层，点击关闭 -->
    <!-- <div v-if="sidebarOpen && isNarrow" class="sidebar-mask" @click="sidebarOpen = false"></div> -->

    <aside class="sidebar">
      <header class="header">
        <el-button v-if="isDesktop() && isNarrow" class="header-btn" @click="manageSites">
          <el-icon class="el-icon--left">
            <House />
          </el-icon>
          站点管理
        </el-button>
        <el-dropdown v-if="isNarrow" trigger="click" @command="(c) => runGenerate(c as 'full' | 'increment')">
          <el-button>
            生成网站
            <el-icon class="el-icon--right">
              <ArrowDown />
            </el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="increment">增量生成</el-dropdown-item>
              <el-dropdown-item command="full">全量生成</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <button class="menu-toggle" @click="toggleSidebar">
          <span class="hamburger">
            <i></i>
            <i></i>
            <i></i>
          </span>
        </button>
      </header>

      <ContentTree class="tree" />

      <nav class="fixed-nav">

        <router-link v-for="item in fixedEntries" :key="item.path" :to="item.path" class="nav-item"
          :class="{ active: activePath === item.path }" :title="item.label" @click="activePath = item.path">
          <el-icon class="nav-icon">
            <component :is="item.icon" />
          </el-icon>
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </nav>
    </aside>

    <main class="main">
      <router-view v-slot="{ Component }">
        <transition name="page" mode="out-in">
          <component :is="Component" :key="route.fullPath" />
        </transition>
      </router-view>
    </main>

    <TaskProgressDialog v-model="genDialog" :task-id="taskId" @done="onGenDone" />
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 380px 1fr;
  grid-template-areas:
    'sidebar main';
  height: 100vh;
  overflow: hidden;
  transition: grid-template-columns 0.2s ease;
}

/* 桌面端侧栏收起为图标条，编辑区获得更多宽度 */
.layout.sidebar-collapsed {
  grid-template-columns: 56px 1fr;
}

.layout.sidebar-collapsed .tree {
  display: none;
}

.layout.sidebar-collapsed .fixed-nav {
  order: 0;
  grid-template-columns: 1fr;
  gap: 10px;
  border-top: none;
}

.layout.sidebar-collapsed .nav-item {
  justify-content: center;
  padding: 10px 0;
}

.layout.sidebar-collapsed .nav-label {
  display: none;
}

.menu-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  border: none;
  background: #f5f7fb;
  cursor: pointer;
  touch-action: manipulation;
  transition: background 0.15s ease;
}

.menu-toggle:hover {
  background: #e8eef9;
}

.hamburger {
  display: inline-flex;
  flex-direction: column;
  gap: 4px;
}

.hamburger i {
  width: 18px;
  height: 2px;
  background: #374151;
  border-radius: 2px;
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.hamburger.open i:nth-child(1) {
  transform: translateY(6px) rotate(45deg);
}

.hamburger.open i:nth-child(2) {
  opacity: 0;
}

.hamburger.open i:nth-child(3) {
  transform: translateY(-6px) rotate(-45deg);
}

.header {
  background: #fff;
  border-bottom: 1px solid #eef0f4;
  box-shadow: 0 1px 0 rgba(16, 24, 40, 0.02), 0 6px 18px -12px rgba(16, 24, 40, 0.12);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px;
  position: relative;
  z-index: 60;
}

.brand-mark {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  background: linear-gradient(135deg, #3b82f6, #1d4ed8);
  box-shadow: 0 4px 10px -2px rgba(37, 99, 235, 0.45);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
}

.brand-name {
  font-weight: 700;
  font-size: 16px;
  letter-spacing: 0.2px;
}

.brand-site {
  color: #6b7280;
  font-size: 13px;
  border-left: 1px solid #e5e7eb;
  padding-left: 10px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-btn {
  height: 32px;
  padding: 0 12px;
  font-size: 13px;
}

.primary-btn {
  font-weight: 600;
}

.sidebar {
  grid-area: sidebar;
  background: #fff;
  border-right: 1px solid #eef0f4;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding-bottom: 12px;
}

/* 内容树是最高频操作区，占据侧栏主高度并独立滚动 */
.tree {
  order: 1;
  flex: 1 1 auto;
  min-height: 0;
}

/* 系统导航压缩到底部，两列紧凑排布，不再挤占内容树 */
.fixed-nav {
  order: 2;
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
  padding: 10px;
  border-top: 1px solid #eef0f4;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 7px 8px;
  border-radius: 7px;
  color: #41536f;
  font-size: 14px;
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease;
  background-color: #f3f6fc;
}

.nav-icon {
  font-size: 15px;
  flex: 0 0 auto;
  color: #9ca3af;
  transition: color 0.15s ease;
}

.nav-item.active .nav-icon {
  color: var(--el-color-primary);
}

.nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-item:hover {
  background: #eaeaea;
  color: #5b8def;
}

.nav-item:hover .nav-icon {
  color: #5b8def;
}

.nav-item.active {
  background: #eef3fe;
  color: var(--el-color-primary);
  font-weight: 600;
}

/* 入口为奇数时最后一项占满整行，避免底部出现半行空缺 */
.nav-item:last-child:nth-child(odd) {
  grid-column: 1 / -1;
}

.main {
  grid-area: main;
  overflow: auto;
  background: #f5f7fb;
}

/* 页面切换过渡：轻量淡入上移，避免生硬跳变 */
.page-enter-active,
.page-leave-active {
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.page-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media screen {}
</style>
