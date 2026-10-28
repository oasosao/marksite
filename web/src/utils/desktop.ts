// 桌面端桥接：在 Wails 客户端里使用系统对话框保存文件、用系统浏览器打开页面、管理站点。
// 浏览器环境下这些能力不存在，调用方应保持原有 Web 实现。

import { Desktop } from '../bindings/marksite/desktop'
import type { WorkDirInfo } from '../bindings/marksite/desktop'

export type { WorkDirInfo }

// v3 桌面端页面由 WebView 通过自定义协议加载（macOS/Linux 为 wails://，Windows 为
// http://wails.localhost），浏览器访问本地服务时不会命中这两种来源，据此判断即可。
export function isDesktop(): boolean {
  if (typeof window === 'undefined') return false
  const { protocol, hostname } = window.location
  return (protocol === 'wails:' || hostname === 'wails.localhost')
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result || '')
      // readAsDataURL 返回形如 data:application/zip;base64,xxxx
      resolve(result.slice(result.indexOf(',') + 1))
    }
    reader.onerror = () => reject(reader.error || new Error('读取文件失败'))
    reader.readAsDataURL(blob)
  })
}

// 桌面端：弹系统保存对话框并写入文件；返回用户取消时为空字符串
export async function desktopSaveFile(filename: string, blob: Blob): Promise<string> {
  if (!isDesktop()) throw new Error('桌面端能力不可用')
  const path = await Desktop.SaveFile(filename, await blobToBase64(blob))
  return path || ''
}

// 桌面端：在系统浏览器打开本地服务路径
export function desktopOpenInBrowser(path: string): boolean {
  if (!isDesktop()) return false
  void Desktop.OpenInBrowser(path)
  return true
}

// 桌面端：读取当前工作目录信息；浏览器环境返回 null
export async function desktopWorkDirInfo(): Promise<WorkDirInfo | null> {
  if (!isDesktop()) return null
  return Desktop.WorkDirInfo()
}

// 桌面端：关闭当前站点并返回站点管理页；成功后窗口会重新加载
export async function desktopShowSites(): Promise<void> {
  if (!isDesktop()) throw new Error('桌面端能力不可用')
  await Desktop.ShowSites()
}
