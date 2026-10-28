import type { TreeNode } from '../types'

export interface FlatDir {
  value: number
  label: string
  depth: number
}

// 拍平分类用于移动选择
export function flattenDirs(nodes: TreeNode[]): FlatDir[] {
  const out: FlatDir[] = []
  const walk = (list: TreeNode[], depth: number) => {
    list
      .filter((n) => n.type === 'dir')
      .forEach((n) => {
        out.push({ value: n.id, label: n.label, depth })
        if (n.children?.length) walk(n.children, depth + 1)
      })
  }
  walk(nodes, 0)
  return out
}
