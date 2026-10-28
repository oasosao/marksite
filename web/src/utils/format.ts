export function formatDate(value: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export function formatSize(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

// 任务类型的中文名，避免界面直接暴露 full/increment 这类内部标识
export function taskTypeText(type: string): string {
  const map: Record<string, string> = {
    full: '全量生成',
    increment: '增量生成',
    fts_rebuild: '重建索引',
  }
  return map[type] || type
}

export function toLocalInput(value: string): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function toRFC3339(value: string): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString()
}

export function slugify(title: string): string {
  // 与后端 pkg.Slugify 对纯 ASCII 输入保持一致；含中文时返回空串，
  // 留给后端用 go-pinyin 自动生成拼音 slug。
  const out = title
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s_.-]/g, '')
    .replace(/[\s_.]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
  return out
}
