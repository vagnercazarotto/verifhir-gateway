import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { AuditEntry } from '../types'

const STAGES = ['', 'ingest', 'parse', 'map', 'score', 'route', 'deliver', 'store']

const STAGE_BADGE: Record<string, string> = {
  ingest:  'bg-blue-100 text-blue-700',
  parse:   'bg-purple-100 text-purple-700',
  map:     'bg-indigo-100 text-indigo-700',
  score:   'bg-yellow-100 text-yellow-700',
  route:   'bg-teal-100 text-teal-700',
  deliver: 'bg-emerald-100 text-emerald-700',
  store:   'bg-slate-100 text-slate-700',
}

const STATUS_BADGE: Record<string, string> = {
  ok:            'bg-green-100 text-green-700',
  error:         'bg-red-100 text-red-700',
  sent:          'bg-emerald-100 text-emerald-700',
  pending:       'bg-violet-100 text-violet-700',
  failed:        'bg-red-100 text-red-700',
  dead_lettered: 'bg-orange-100 text-orange-700',
  skipped:       'bg-gray-100 text-gray-600',
  no_channels:   'bg-amber-100 text-amber-700',
}

function StageBadge({ stage }: { stage: string }) {
  return (
    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${STAGE_BADGE[stage] ?? 'bg-gray-100 text-gray-600'}`}>
      {stage}
    </span>
  )
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_BADGE[status] ?? 'bg-gray-100 text-gray-600'}`}>
      {status}
    </span>
  )
}

// ---- expandable row --------------------------------------------------------

function EntryRow({ e }: { e: AuditEntry }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <tr
        className="hover:bg-gray-50 cursor-pointer"
        onClick={() => setOpen(o => !o)}
      >
        <td className="px-4 py-2 text-xs text-gray-500 whitespace-nowrap font-mono">
          {new Date(e.ts).toLocaleString()}
        </td>
        <td className="px-4 py-2">
          <StageBadge stage={e.stage} />
        </td>
        <td className="px-4 py-2">
          <StatusBadge status={e.status} />
        </td>
        <td className="px-4 py-2 font-mono text-xs text-gray-600 truncate max-w-[160px]">
          {e.msg_id}
        </td>
        <td className="px-4 py-2 text-xs text-gray-500">{e.duration_ms ?? '—'} ms</td>
        <td className="px-4 py-2 text-xs text-red-600 truncate max-w-[200px]">{e.error ?? ''}</td>
        <td className="px-4 py-2 text-xs text-gray-400 text-center">{open ? '▲' : '▼'}</td>
      </tr>
      {open && (
        <tr className="bg-gray-50">
          <td colSpan={7} className="px-6 py-3">
            <dl className="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-1 text-xs">
              {e.resource_type && <><dt className="text-gray-400">Resource</dt><dd>{e.resource_type}</dd></>}
              {e.event_type    && <><dt className="text-gray-400">Event</dt><dd>{e.event_type}</dd></>}
              {e.segments      && <><dt className="text-gray-400">Segments</dt><dd>{e.segments}</dd></>}
              {e.score         != null && <><dt className="text-gray-400">Score</dt><dd>{e.score.toFixed(3)}</dd></>}
              {e.completeness  != null && <><dt className="text-gray-400">Completeness</dt><dd>{e.completeness.toFixed(3)}</dd></>}
              {e.conformity    != null && <><dt className="text-gray-400">Conformity</dt><dd>{e.conformity.toFixed(3)}</dd></>}
              {e.confidence    != null && <><dt className="text-gray-400">Confidence</dt><dd>{e.confidence.toFixed(3)}</dd></>}
              {e.findings      != null && <><dt className="text-gray-400">Findings</dt><dd>{e.findings}</dd></>}
              {e.error         && <><dt className="text-gray-400">Error</dt><dd className="text-red-600 break-all col-span-2">{e.error}</dd></>}
            </dl>
          </td>
        </tr>
      )}
    </>
  )
}

// ---- page ------------------------------------------------------------------

export default function AuditLog() {
  const [entries, setEntries]   = useState<AuditEntry[]>([])
  const [loading, setLoading]   = useState(true)
  const [error, setError]       = useState<string | null>(null)
  const [stage, setStage]       = useState('')
  const [from, setFrom]         = useState('')
  const [to, setTo]             = useState('')
  const [limit, setLimit]       = useState(200)

  function load() {
    setLoading(true)
    setError(null)
    api.audit
      .list({ stage: stage || undefined, from: from || undefined, to: to || undefined, limit })
      .then(r => setEntries(r ?? []))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="p-6 space-y-4">
      {/* header */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-xl font-semibold text-gray-800">Audit Log</h1>
        <button
          onClick={load}
          className="px-3 py-1.5 rounded-md border border-gray-300 text-sm hover:bg-gray-50"
        >
          Refresh
        </button>
      </div>

      {/* filters */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4 flex flex-wrap gap-3 items-end">
        <label className="flex flex-col gap-1 text-xs text-gray-500">
          Stage
          <select
            value={stage}
            onChange={e => setStage(e.target.value)}
            className="rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {STAGES.map(s => <option key={s} value={s}>{s || 'All stages'}</option>)}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-xs text-gray-500">
          From
          <input
            type="datetime-local"
            value={from}
            onChange={e => setFrom(e.target.value ? new Date(e.target.value).toISOString() : '')}
            className="rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </label>

        <label className="flex flex-col gap-1 text-xs text-gray-500">
          To
          <input
            type="datetime-local"
            value={to}
            onChange={e => setTo(e.target.value ? new Date(e.target.value).toISOString() : '')}
            className="rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </label>

        <label className="flex flex-col gap-1 text-xs text-gray-500">
          Limit
          <select
            value={limit}
            onChange={e => setLimit(Number(e.target.value))}
            className="rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {[50, 100, 200, 500, 1000].map(n => <option key={n} value={n}>{n}</option>)}
          </select>
        </label>

        <button
          onClick={load}
          className="px-4 py-1.5 rounded-md bg-brand-600 text-white text-sm hover:bg-brand-700"
        >
          Apply
        </button>
      </div>

      {/* table */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-x-auto">
        {loading && (
          <div className="p-8 text-center text-gray-400 text-sm">Loading…</div>
        )}
        {!loading && error && (
          <div className="p-8 text-center text-red-500 text-sm">{error}</div>
        )}
        {!loading && !error && entries.length === 0 && (
          <div className="p-8 text-center text-gray-400 text-sm">No audit entries found.</div>
        )}
        {!loading && !error && entries.length > 0 && (
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500">Time</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500">Stage</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500">Status</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500">Message ID</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500">Duration</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500">Error</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {entries.map((e, i) => <EntryRow key={`${e.msg_id}-${e.stage}-${i}`} e={e} />)}
            </tbody>
          </table>
        )}
      </div>

      <p className="text-xs text-gray-400">
        Showing {entries.length} entries · click a row to expand detail
      </p>
    </div>
  )
}
