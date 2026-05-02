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
