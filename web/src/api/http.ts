import axios, { type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { desktopSaveFile, isDesktop } from '../utils/desktop'

interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

const instance = axios.create({
  baseURL: '/api/v1',
  timeout: 60000,
})

instance.interceptors.request.use((config) => {
  // 桌面端 WebView 会缓存同 URL 的 GET 响应，加上时间戳确保每次拿到最新数据
  if ((config.method || 'get').toLowerCase() === 'get') {
    config.params = { ...(config.params || {}), _t: Date.now() }
  }
  return config
})

instance.interceptors.response.use(
  (resp) => {
    if (resp.config.responseType === 'blob') {
      return resp
    }
    const body = resp.data as ApiResponse<unknown>
    if (body && body.code !== 0) {
      ElMessage.error(body.message || '请求失败')
      return Promise.reject(new Error(body.message || '请求失败'))
    }
    // 直接返回业务数据，响应拦截器对外约定为解包后的 data
    return body.data as never
  },
  (error) => {
    const data = error.response?.data as ApiResponse<unknown> | undefined
    const message = data?.message || error.message || '网络错误'
    ElMessage.error(message)
    return Promise.reject(new Error(message))
  },
)

export function request<T>(config: AxiosRequestConfig): Promise<T> {
  return instance.request(config) as unknown as Promise<T>
}

export function download(config: AxiosRequestConfig, filename: string) {
  return (instance.request({ ...config, responseType: 'blob' }) as unknown as Promise<{ data: Blob }>)
    .then((resp) => {
      // 后端返回 JSON 错误时，响应体仍是 application/json 而非 blob
      const data = resp.data
      if (data instanceof Blob && data.type.includes('application/json')) {
        return data.text().then((text) => {
          try {
            const body = JSON.parse(text) as ApiResponse<unknown>
            ElMessage.error(body.message || '下载失败')
          } catch {
            ElMessage.error('下载失败')
          }
          return Promise.reject(new Error('下载失败'))
        })
      }
      // 桌面端 webview 不支持 a[download]，改用系统保存对话框
      if (isDesktop()) {
        return desktopSaveFile(filename, data).then((path) => {
          if (path) ElMessage.success(`已保存到 ${path}`)
        })
      }
      const url = URL.createObjectURL(data)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    })
    .catch((err) => {
      ElMessage.error(err?.message || '下载失败')
      return Promise.reject(err)
    })
}
