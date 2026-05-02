// Thin fetch wrapper for the Gateway REST API.
// During local dev Vite proxies /api, /healthz, /readyz to localhost:8080.
// In production nginx does the same proxy.

import type { AuditEntry, Channel, HealthResponse, IngestResponse, Message, MessageStatus, Pipeline, ReportSummary, Source } from '../types'

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

  sources: {
    list: () => request<Source[]>('/api/v1/sources'),
    get: (id: string) => request<Source>(`/api/v1/sources/${id}`),
    create: (src: Omit<Source, 'created_at' | 'updated_at'>) =>
      request<Source>('/api/v1/sources', {
        method: 'POST',
        body: JSON.stringify(src),
      }),
    update: (id: string, src: Partial<Omit<Source, 'created_at' | 'updated_at'>>) =>
      request<Source>(`/api/v1/sources/${id}`, {
        method: 'PUT',
        body: JSON.stringify(src),
      }),
    delete: (id: string) =>
      request<void>(`/api/v1/sources/${id}`, { method: 'DELETE' }),
  },

  pipelines: {
    list: () => request<Pipeline[]>('/api/v1/pipelines'),
    get: (id: string) => request<Pipeline>(`/api/v1/pipelines/${id}`),
    create: (p: Omit<Pipeline, 'created_at' | 'updated_at'>) =>
      request<Pipeline>('/api/v1/pipelines', {
        method: 'POST',
        body: JSON.stringify(p),
      }),
    update: (id: string, p: Partial<Omit<Pipeline, 'created_at' | 'updated_at'>>) =>
      request<Pipeline>(`/api/v1/pipelines/${id}`, {
        method: 'PUT',
        body: JSON.stringify(p),
      }),
    delete: (id: string) =>
      request<void>(`/api/v1/pipelines/${id}`, { method: 'DELETE' }),
  },

  ingest: {
    send: (hl7v2: string) =>
      request<IngestResponse>('/api/v1/ingest', {
        method: 'POST',
        headers: { 'Content-Type': 'text/plain' },
        body: hl7v2,
      }),
  },

  health: {
    liveness: () => request<HealthResponse>('/healthz'),
    readiness: () => request<HealthResponse>('/readyz'),
  },

  audit: {
    list: (params?: { from?: string; to?: string; stage?: string; limit?: number }) => {
      const q = new URLSearchParams()
      if (params?.from)  q.set('from',  params.from)
      if (params?.to)    q.set('to',    params.to)
      if (params?.stage) q.set('stage', params.stage)
      if (params?.limit != null) q.set('limit', String(params.limit))
      return request<AuditEntry[]>(`/api/v1/audit?${q}`)
    },
  },

  reports: {
    summary: (params?: { from?: string; to?: string }) => {
      const q = new URLSearchParams()
      if (params?.from) q.set('from', params.from)
      if (params?.to)   q.set('to',   params.to)
      return request<ReportSummary>(`/api/v1/reports?${q}`)
    },
  },
}
