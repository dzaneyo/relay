import type { RecordDetail, RecordInput, RecordSummary, RouteInput, SSHRoute } from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: init?.body ? { 'Content-Type': 'application/json', ...init.headers } : init?.headers,
  })

  if (!response.ok) {
    let message = `Request failed (${response.status})`
    try {
      const body = (await response.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // Keep the status-based fallback when a response is not JSON.
    }
    throw new Error(message)
  }

  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function listRecords(query = ''): Promise<RecordSummary[]> {
  const suffix = query.trim() ? `?q=${encodeURIComponent(query.trim())}` : ''
  return request(`/api/records${suffix}`)
}

export function getRecord(id: string): Promise<RecordDetail> {
  return request(`/api/records/${encodeURIComponent(id)}`)
}

export function createRecord(input: RecordInput): Promise<RecordDetail> {
  return request('/api/records', { method: 'POST', body: JSON.stringify(input) })
}

export function updateRecord(id: string, input: RecordInput): Promise<RecordDetail> {
  return request(`/api/records/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(input) })
}

export function deleteRecord(id: string): Promise<void> {
  return request(`/api/records/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function listRoutes(): Promise<SSHRoute[]> {
  return request('/api/routes')
}

export function createRoute(input: RouteInput): Promise<SSHRoute> {
  return request('/api/routes', { method: 'POST', body: JSON.stringify(input) })
}

export function updateRoute(id: string, input: RouteInput): Promise<SSHRoute> {
  return request(`/api/routes/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(input) })
}

export function deleteRoute(id: string): Promise<void> {
  return request(`/api/routes/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
