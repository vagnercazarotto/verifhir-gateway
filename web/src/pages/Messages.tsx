import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { Message, MessageStatus } from '../types'

const STATUS_BADGE: Record<MessageStatus, string> = {
  pending:       'bg-violet-100 text-violet-700',
  sent:          'bg-green-100  text-green-700',
  failed:        'bg-red-100    text-red-700',
  dead_lettered: 'bg-orange-100 text-orange-700',
}

const STATUSES: Array<MessageStatus | ''> = ['', 'pending', 'sent', 'failed', 'dead_lettered']

function Badge({ status }: { status: MessageStatus }) {
  return (
    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_BADGE[status]}`}>
      {status.replace('_', ' ')}
    </span>
  )
}

function ScoreBar({ value }: { value: number }) {
  const pct = Math.round(value * 100)
  const color = value >= 0.9 ? 'bg-green-400' : value >= 0.7 ? 'bg-yellow-400' : 'bg-red-400'
  return (
    <div className="flex items-center gap-2">
      <div className="w-20 h-1.5 rounded-full bg-gray-200 overflow-hidden">
        <div className={`h-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs text-gray-500">{value.toFixed(3)}</span>
    </div>
  )
}

// ---- detail panel ----------------------------------------------------------

function DetailPanel({ id, onClose }: { id: string; onClose: () => void }) {
  const [msg, setMsg] = useState<Message | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.messages.get(id).then(setMsg).catch((e: Error) => setError(e.message))
  }, [id])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">Message Detail</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none">×</button>
        </div>

        {!msg && !error && <p className="text-gray-400 text-sm">Loading…</p>}
        {error && <p className="text-red-500 text-sm">{error}</p>}

        {msg && (
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
            <dt className="text-gray-500">ID</dt>
            <dd className="font-mono text-xs break-all">{msg.id}</dd>

            <dt className="text-gray-500">Resource Type</dt>
            <dd>{msg.resource_type}</dd>

            <dt className="text-gray-500">Status</dt>
            <dd><Badge status={msg.status} /></dd>

            <dt className="text-gray-500">Attempts</dt>
            <dd>{msg.attempts}</dd>

            <dt className="text-gray-500">Quality Score</dt>
            <dd><ScoreBar value={msg.quality_score} /></dd>

            <dt className="text-gray-500">Completeness</dt>
            <dd><ScoreBar value={msg.completeness} /></dd>

            <dt className="text-gray-500">Conformity</dt>
            <dd><ScoreBar value={msg.conformity} /></dd>

            <dt className="text-gray-500">Confidence</dt>
            <dd><ScoreBar value={msg.confidence} /></dd>

            <dt className="text-gray-500">Created</dt>
            <dd>{new Date(msg.created_at).toLocaleString()}</dd>

            <dt className="text-gray-500">Updated</dt>
            <dd>{new Date(msg.updated_at).toLocaleString()}</dd>

            {msg.last_error && (
              <>
                <dt className="text-gray-500">Last Error</dt>
                <dd className="text-red-600 text-xs break-all">{msg.last_error}</dd>
              </>
            )}
          </dl>
        )}
      </div>
    </div>
  )
}

// ---- page ------------------------------------------------------------------

export default function Messages() {
  const { id: routeId } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState<MessageStatus | ''>('')
  const [limit, setLimit] = useState(100)
  const [detailId, setDetailId] = useState<string | null>(routeId ?? null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    api.messages
      .list({ status: statusFilter || undefined, limit })
      .then(msgs => setMessages(msgs ?? []))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [statusFilter, limit])

  function openDetail(id: string) {
    setDetailId(id)
    navigate(`/messages/${id}`, { replace: true })
  }

  function closeDetail() {
    setDetailId(null)
    navigate('/messages', { replace: true })
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-xl font-semibold text-gray-800">Message History</h1>

        <div className="flex items-center gap-3">
          {/* status filter */}
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value as MessageStatus | '')}
            className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {STATUSES.map(s => (
              <option key={s} value={s}>{s || 'All statuses'}</option>
            ))}
          </select>

          {/* limit */}
          <select
            value={limit}
            onChange={e => setLimit(Number(e.target.value))}
            className="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {[50, 100, 250, 500].map(n => (
              <option key={n} value={n}>{n} rows</option>
            ))}
          </select>
        </div>
      </div>

      {loading && <p className="text-gray-400 text-sm">Loading…</p>}
      {error && <p className="text-red-500 text-sm">{error}</p>}

      {!loading && !error && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                {['ID', 'Type', 'Score', 'Status', 'Attempts', 'Created'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {messages.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-gray-400">
                    No messages found.
                  </td>
                </tr>
              )}
              {messages.map(m => (
                <tr
                  key={m.id}
                  className="hover:bg-gray-50 cursor-pointer"
                  onClick={() => openDetail(m.id)}
                >
                  <td className="px-4 py-2 font-mono text-xs text-gray-500">{m.id}</td>
                  <td className="px-4 py-2">{m.resource_type}</td>
                  <td className="px-4 py-2"><ScoreBar value={m.quality_score} /></td>
                  <td className="px-4 py-2"><Badge status={m.status} /></td>
                  <td className="px-4 py-2 text-gray-500">{m.attempts}</td>
                  <td className="px-4 py-2 text-gray-400 text-xs">
                    {new Date(m.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {detailId && <DetailPanel id={detailId} onClose={closeDetail} />}
    </div>
  )
}
