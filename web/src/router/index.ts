import { createRouter, createWebHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: MainLayout,
      redirect: '/dashboard',
      children: [
        { path: 'dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
        { path: 'posts', name: 'posts', component: () => import('../views/PostList.vue') },
        { path: 'posts/edit/:id', name: 'post-edit', component: () => import('../views/PostEditor.vue') },
        { path: 'pages', name: 'pages', component: () => import('../views/PageList.vue') },
        { path: 'pages/edit/:id', name: 'page-edit', component: () => import('../views/PageEditor.vue') },
        { path: 'dirs/edit/:id', name: 'dir-edit', component: () => import('../views/DirEditor.vue') },
        { path: 'tags', name: 'tags', component: () => import('../views/TagManager.vue') },
        { path: 'assets', name: 'assets', component: () => import('../views/Assets.vue') },
        { path: 'themes', name: 'themes', component: () => import('../views/Themes.vue') },
        { path: 'setting/site', name: 'site-setting', component: () => import('../views/SiteSetting.vue') },
        { path: 'setting/app', name: 'app-setting', component: () => import('../views/AppSetting.vue') },
        { path: 'backup', name: 'backup', component: () => import('../views/Backup.vue') },
        { path: 'fts-index', name: 'fts-index', component: () => import('../views/FtsIndex.vue') },
        { path: 'preview', name: 'preview', component: () => import('../views/Preview.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('../views/NotFound.vue') },
  ],
})

export default router
