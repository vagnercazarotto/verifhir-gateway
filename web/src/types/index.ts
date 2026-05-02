// Shared domain types mirroring the Go REST API responses.

export type MessageStatus = 'pending' | 'sent' | 'failed' | 'dead_lettered'

export interface Message {
  id: string
  resource_type: string
  quality_score: number
  completeness: number
  conformity: number
  confidence: number
  status: MessageStatus
  attempts: number
  last_error?: string
  created_at: string
  updated_at: string
}

export interface RetryConfig {
  max_attempts: number
  initial_backoff_ms: number
  multiplier: number
}

export interface Channel {
  id: string
  name: string
  url: string
  auth_header?: string
  timeout_ms: number
  min_quality_score: number
  enabled: boolean
  retry: RetryConfig
  created_at: string
  updated_at: string
}

export interface HealthResponse {
  status: 'ok' | 'unavailable'
  reason?: string
}

// ---- audit -----------------------------------------------------------------

export interface AuditEntry {
  ts: string
  msg_id: string
  stage: string
  duration_ms: number
  status: string
  error?: string
  resource_type?: string
  event_type?: string
  segments?: number
  score?: number
  completeness?: number
  conformity?: number
  confidence?: number
  findings?: number
  channel_id?: string
  dest_url?: string
}

// ---- reports ---------------------------------------------------------------

export interface DaySummary {
  date: string
  total: number
  avg_score: number
  sent: number
  failed: number
  dead_lettered: number
  pending: number
}

export interface ReportSummary {
  from: string
  to: string
  total: number
  avg_score: number
  by_status: Record<string, number>
  by_day: DaySummary[]
}
