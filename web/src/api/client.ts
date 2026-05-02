// Thin fetch wrapper for the Gateway REST API.
// During local dev Vite proxies /api, /healthz, /readyz to localhost:8080.
// In production nginx does the same proxy.

import type { Channel, HealthResponse, Message, MessageStatus } from '../types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const body: { error?: string } = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  // DELETE returns 204 No Content — skip JSON parsing
  if (res.status === 204) return undefined as unknown as T
  return res.json() as Promise<T>
}

export const api = {
  messages: {
    list: (params?: { status?: MessageStatus; limit?: number }) => {
      const q = new URLSearchParams()
      if (params?.status) q.set('status', params.status)
      if (params?.limit != null) q.set('limit', String(params.limit))
      return request<Message[]>(`/api/v1/messages?${q}`)
    },
    get: (id: string) => request<Message>(`/api/v1/messages/${id}`),
  },

  channels: {
    list: () => request<Channel[]>('/api/v1/channels'),
    get: (id: string) => request<Channel>(`/api/v1/channels/${id}`),
    create: (ch: Omit<Channel, 'created_at' | 'updated_at'>) =>
      request<Channel>('/api/v1/channels', {
        method: 'POST',
        body: JSON.stringify(ch),
      }),
    update: (id: string, ch: Partial<Omit<Channel, 'created_at' | 'updated_at'>>) =>
      request<Channel>(`/api/v1/channels/${id}`, {
        method: 'PUT',
        body: JSON.stringify(ch),
      }),
    delete: (id: string) =>
      request<void>(`/api/v1/channels/${id}`, { method: 'DELETE' }),
  },

  health: {
    liveness: () => request<HealthResponse>('/healthz'),
    readiness: () => request<HealthResponse>('/readyz'),
  },
}
