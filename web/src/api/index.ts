import { download, request } from './http'
import type {
  AppConfig, Asset, BackupInfo, Directory, Page, Paged, Post, SearchResult,
  SiteConfig, SnapshotMeta, Stats, Tag, TagCount, Task, ThemeInfo, TreeNode,
} from '../types'

export interface PostInput {
  dir_id: number
  title: string
  slug: string
  content: string
  summary?: string
  cover?: string
  tags?: string[]
  sort?: number
  is_published?: number
  published_at?: string
}

export interface PageInput {
  dir_id: number
  title: string
  slug: string
  content: string
  summary?: string
  cover?: string
  tags?: string[]
  sort?: number
  is_published?: number
  published_at?: string
}

export interface DirInput {
  parent_id: number
  name: string
  slug?: string
  page_title?: string
  page_content?: string
  page_summary?: string
  cover?: string
  is_page?: number
  is_published?: number
  menu_visible?: number
}

export type PostUpdate = Partial<PostInput>
export type PageUpdate = Partial<PageInput>
export type DirUpdate = Partial<DirInput>

export const statsApi = {
  get: () => request<Stats>({ url: '/stats', method: 'get' }),
}

export const treeApi = {
  get: () => request<TreeNode[]>({ url: '/tree', method: 'get' }),
  createDir: (data: DirInput) =>
    request<Directory>({ url: '/dirs', method: 'post', data }),
  updateDir: (id: number, data: DirUpdate) =>
    request<Directory>({ url: `/dirs/${id}`, method: 'put', data }),
  getDir: (id: number) => request<Directory>({ url: `/dirs/${id}`, method: 'get' }),
  deleteDir: (id: number, mode: 'move' | 'cascade') =>
    request<Record<string, number>>({ url: `/dirs/${id}`, method: 'delete', params: { mode } }),
  movePost: (id: number, dir_id: number) =>
    request<Post>({ url: `/posts/${id}/move`, method: 'put', data: { dir_id } }),
  movePage: (id: number, dir_id: number) =>
    request<Page>({ url: `/pages/${id}/move`, method: 'put', data: { dir_id } }),
  sort: (parent_id: number, items: { type: string; id: number; sort: number }[]) =>
    request<null>({ url: '/tree/sort', method: 'put', data: { parent_id, items } }),
}

export const tagApi = {
  list: () => request<TagCount[]>({ url: '/tags', method: 'get' }),
  create: (data: { name: string; slug?: string; sort?: number }) =>
    request<Tag>({ url: '/tags', method: 'post', data }),
  update: (id: number, data: { name?: string; slug?: string; sort?: number }) =>
    request<Tag>({ url: `/tags/${id}`, method: 'put', data }),
  remove: (id: number) => request<null>({ url: `/tags/${id}`, method: 'delete' }),
}

export const postApi = {
  list: (params: Record<string, unknown>) =>
    request<Paged<Post>>({ url: '/posts', method: 'get', params }),
  get: (id: number) => request<Post>({ url: `/posts/${id}`, method: 'get' }),
  create: (data: PostInput) =>
    request<Post>({ url: '/posts', method: 'post', data }),
  update: (id: number, data: PostUpdate) =>
    request<Post>({ url: `/posts/${id}`, method: 'put', data }),
  remove: (id: number) => request<null>({ url: `/posts/${id}`, method: 'delete' }),
  exportOne: (id: number, filename: string) =>
    download({ url: `/posts/${id}/export`, method: 'get' }, filename),
  batchExport: (ids: number[]) =>
    download({ url: '/posts/batch-export', method: 'post', data: { ids } }, 'posts-export.zip'),
  importZip: (file: File, dir_id: number) => {
    const form = new FormData()
    form.append('file', file)
    form.append('dir_id', String(dir_id))
    return request<{ imported: number; skipped: number; errors: { file: string; reason: string }[] }>({
      url: '/posts/import', method: 'post', data: form,
    })
  },
  snapshots: (id: number) => request<SnapshotMeta[]>({ url: `/posts/${id}/snapshots`, method: 'get' }),
  rollback: (id: number, snapshot_id: number) =>
    request<Post>({ url: `/posts/${id}/rollback`, method: 'post', data: { snapshot_id } }),
}

export const pageApi = {
  list: (params: Record<string, unknown>) =>
    request<Paged<Page>>({ url: '/pages', method: 'get', params }),
  get: (id: number) => request<Page>({ url: `/pages/${id}`, method: 'get' }),
  create: (data: PageInput) =>
    request<Page>({ url: '/pages', method: 'post', data }),
  update: (id: number, data: PageUpdate) =>
    request<Page>({ url: `/pages/${id}`, method: 'put', data }),
  remove: (id: number) => request<null>({ url: `/pages/${id}`, method: 'delete' }),
  exportOne: (id: number, filename: string) =>
    download({ url: `/pages/${id}/export`, method: 'get' }, filename),
  batchExport: (ids: number[]) =>
    download({ url: '/pages/batch-export', method: 'post', data: { ids } }, 'pages-export.zip'),
  importZip: (file: File, dir_id: number) => {
    const form = new FormData()
    form.append('file', file)
    form.append('dir_id', String(dir_id))
    return request<{ imported: number; skipped: number; errors: { file: string; reason: string }[] }>({
      url: '/pages/import', method: 'post', data: form,
    })
  },
  snapshots: (id: number) => request<SnapshotMeta[]>({ url: `/pages/${id}/snapshots`, method: 'get' }),
  rollback: (id: number, snapshot_id: number) =>
    request<Page>({ url: `/pages/${id}/rollback`, method: 'post', data: { snapshot_id } }),
}

export const assetApi = {
  list: (params: Record<string, unknown>) =>
    request<Paged<Asset>>({ url: '/assets', method: 'get', params }),
  upload: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<Asset>({ url: '/assets/upload', method: 'post', data: form })
  },
  remove: (id: number) => request<null>({ url: `/assets/${id}`, method: 'delete' }),
  cleanUnused: () =>
    request<{ scanned: number; deleted_count: number; freed_bytes: number }>({
      url: '/assets/clean-unused', method: 'post',
    }),
}

export const themeApi = {
  list: () => request<ThemeInfo[]>({ url: '/themes', method: 'get' }),
  upload: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<ThemeInfo>({ url: '/themes/upload', method: 'post', data: form })
  },
  activate: (name: string) =>
    request<{ active: string }>({ url: '/themes/active', method: 'put', data: { name } }),
  remove: (name: string) => request<null>({ url: `/themes/${name}`, method: 'delete' }),
}

export const configApi = {
  getSite: () => request<SiteConfig>({ url: '/site-config', method: 'get' }),
  updateSite: (data: SiteConfig) => request<SiteConfig>({ url: '/site-config', method: 'put', data }),
  getApp: () => request<AppConfig>({ url: '/app-config', method: 'get' }),
  updateApp: (data: AppConfig) => request<AppConfig>({ url: '/app-config', method: 'put', data }),
}

export const generateApi = {
  full: () => request<{ task_id: number }>({ url: '/generate/full', method: 'post' }),
  increment: () => request<{ task_id: number }>({ url: '/generate/increment', method: 'post' }),
  tasks: (limit = 20) => request<Task[]>({ url: '/generate/tasks', method: 'get', params: { limit } }),
  task: (task_id: number) =>
    request<Task>({ url: '/generate/tasks', method: 'get', params: { task_id } }),
  cancel: (id: number) =>
    request<{ id: number; status: string }>({ url: `/generate/cancel/${id}`, method: 'post' }),
}

export const backupApi = {
  list: () => request<BackupInfo[]>({ url: '/backups', method: 'get' }),
  create: () => request<{ name: string; size: number }>({ url: '/backups/create', method: 'post' }),
  restore: (name: string) =>
    request<{ restored: boolean; safety_backup: string }>({
      url: `/backups/restore/${name}`, method: 'post',
    }),
  remove: (name: string) => request<null>({ url: `/backups/${name}`, method: 'delete' }),
  download: (name: string) =>
    download({ url: `/backups/${name}/download`, method: 'get' }, name),
}

export const ftsApi = {
  status: () =>
    request<{ exists: boolean; docs: number; size: number; index_path: string }>({
      url: '/fts/status', method: 'get',
    }),
  rebuild: () => request<{ task_id: number }>({ url: '/fts/rebuild', method: 'post' }),
  search: (q: string, limit = 20) =>
    request<SearchResult>({ url: '/search', method: 'get', params: { q, limit } }),
}
