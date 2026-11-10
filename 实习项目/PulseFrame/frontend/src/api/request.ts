// axios 请求封装：自动附带访问令牌，401 时用刷新 Cookie 换取新令牌并重试一次。
import axios, {
  type AxiosError,
  type AxiosRequestConfig,
  type AxiosResponse,
} from 'axios'

let accessToken: string | null = null

export function setAccessToken(token: string | null) {
  accessToken = token
}

export function getAccessToken() {
  return accessToken
}

const client = axios.create({
  baseURL: '',
  timeout: 30000,
  withCredentials: true,
})

client.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }
  return config
})

async function refreshToken(): Promise<boolean> {
  try {
    const res = await axios.post<{ accessToken: string }>(
      '/api/refresh',
      {},
      { withCredentials: true },
    )
    accessToken = res.data.accessToken
    return true
  } catch {
    accessToken = null
    return false
  }
}

function parseError(error: AxiosError) {
  const status = error.response?.status ?? 0
  const data = error.response?.data as
    | { message?: string; code?: string }
    | undefined
  const err = new Error(data?.message || `HTTP ${status}`) as Error & {
    status?: number
    code?: string
  }
  err.status = status
  err.code = data?.code
  return err
}

async function dispatch<T>(config: AxiosRequestConfig): Promise<T> {
  let retried = false
  try {
    const res: AxiosResponse<T> = await client.request<T>(config)
    return res.data
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 401 && !retried) {
      retried = true
      const refreshed = await refreshToken()
      if (!refreshed) throw parseError(error)
      try {
        const res: AxiosResponse<T> = await client.request<T>(config)
        return res.data
      } catch (retryError) {
        if (axios.isAxiosError(retryError)) throw parseError(retryError)
        throw retryError
      }
    }
    if (axios.isAxiosError(error)) throw parseError(error)
    throw error
  }
}

export const api = {
  get<T>(path: string): Promise<T> {
    return dispatch<T>({ url: '/api' + path, method: 'get' })
  },
  post<T>(path: string, body?: unknown): Promise<T> {
    return dispatch<T>({ url: '/api' + path, method: 'post', data: body })
  },
  put<T>(path: string, body?: unknown): Promise<T> {
    return dispatch<T>({ url: '/api' + path, method: 'put', data: body })
  },
  patch<T>(path: string, body?: unknown): Promise<T> {
    return dispatch<T>({ url: '/api' + path, method: 'patch', data: body })
  },
  delete<T>(path: string): Promise<T> {
    return dispatch<T>({ url: '/api' + path, method: 'delete' })
  },
}
