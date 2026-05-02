import { useEffect, useState } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, Legend, ResponsiveContainer,
  PieChart, Pie, Cell,
} from 'recharts'
import { api } from '../api/client'
import type { ReportSummary } from '../types'

const STATUS_COLORS: Record<string, string> = {
  pending:       '#a78bfa',
  sent:          '#34d399',
  failed:        '#f87171',
  dead_lettered: '#fb923c',
}

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">{label}</p>
      <p className="mt-1 text-3xl font-bold text-gray-900">{value}</p>
      {sub && <p className="mt-1 text-sm text-gray-400">{sub}</p>}
    </div>
  )
}

function defaultRange(): { from: string; to: string } {
  const now = new Date()
  const from = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
  return { from: from.toISOString(), to: now.toISOString() }
}

export default function Reports() {
  const [summary, setSummary]   = useState<ReportSummary | null>(null)
  const [loading, setLoading]   = useState(true)
  const [error, setError]       = useState<string | null>(null)
  const [from, setFrom]         = useState(defaultRange().from)
  const [to, setTo]             = useState(defaultRange().to)

  function load() {
    setLoading(true)
    setError(null)
    api.reports
      .summary({ from: from || undefined, to: to || undefined })
      .then(setSummary)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const statusData = summary
    ? Object.entries(summary.by_status).map(([name, value]) => ({ name, value }))
    : []

  const dailyData = (summary?.by_day ?? []).map(d => ({
    date:           d.date,
    sent:           d.sent,
    failed:         d.failed,
    dead_lettered:  d.dead_lettered,
    pending:        d.pending,
    avg_score:      d.avg_score,
  }))

  return (
    <div className="p-6 space-y-4">
      {/* header */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-xl font-semibold text-gray-800">Reports</h1>
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
          From
          <input
            type="datetime-local"
            value={from ? new Date(from).toISOString().slice(0, 16) : ''}
            onChange={e => setFrom(e.target.value ? new Date(e.target.value).toISOString() : '')}
            className="rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </label>

        <label className="flex flex-col gap-1 text-xs text-gray-500">
          To
          <input
            type="datetime-local"
            value={to ? new Date(to).toISOString().slice(0, 16) : ''}
            onChange={e => setTo(e.target.value ? new Date(e.target.value).toISOString() : '')}
            className="rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </label>

        <button
          onClick={load}
          className="px-4 py-1.5 rounded-md bg-brand-600 text-white text-sm hover:bg-brand-700"
        >
          Apply
        </button>
      </div>

      {/* loading / error / empty */}
      {loading && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center text-gray-400 text-sm">
          Loading…
        </div>
      )}
      {!loading && error && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center text-red-500 text-sm">
          {error}
        </div>
      )}
      {!loading && !error && summary && summary.total === 0 && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center text-gray-400 text-sm">
          No messages in the selected range.
        </div>
      )}

      {/* content */}
      {!loading && !error && summary && summary.total > 0 && (
        <>
          {/* stat cards */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard label="Total Messages"    value={summary.total} />
            <StatCard label="Avg Quality Score" value={summary.avg_score.toFixed(3)} />
            <StatCard
              label="Date Range"
              value={`${dailyData.length}d`}
              sub={`${new Date(summary.from).toLocaleDateString()} → ${new Date(summary.to).toLocaleDateString()}`}
            />
            <StatCard
              label="Sent"
              value={summary.by_status.sent ?? 0}
              sub={`${(((summary.by_status.sent ?? 0) / summary.total) * 100).toFixed(0)}% success`}
            />
          </div>

          {/* status pie + daily bar */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
              <p className="text-sm font-medium text-gray-600 mb-3">Status Breakdown</p>
              {statusData.length === 0 ? (
                <div className="flex items-center justify-center h-[220px] text-gray-300 text-sm">No data</div>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <PieChart>
                    <Pie
                      data={statusData}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      outerRadius={80}
                      label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                    >
                      {statusData.map(entry => (
                        <Cell key={entry.name} fill={STATUS_COLORS[entry.name] ?? '#94a3b8'} />
                      ))}
                    </Pie>
                    <Legend />
                  </PieChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
              <p className="text-sm font-medium text-gray-600 mb-3">Daily Volume</p>
              {dailyData.length === 0 ? (
                <div className="flex items-center justify-center h-[220px] text-gray-300 text-sm">No data</div>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={dailyData}>
                    <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                    <YAxis tick={{ fontSize: 11 }} />
                    <Tooltip />
                    <Legend />
                    <Bar dataKey="sent"          stackId="a" fill={STATUS_COLORS.sent} />
                    <Bar dataKey="failed"        stackId="a" fill={STATUS_COLORS.failed} />
                    <Bar dataKey="dead_lettered" stackId="a" fill={STATUS_COLORS.dead_lettered} />
                    <Bar dataKey="pending"       stackId="a" fill={STATUS_COLORS.pending} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          {/* daily table */}
          <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-2 text-left  text-xs font-medium text-gray-500">Date</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-gray-500">Total</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-gray-500">Avg Score</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-gray-500">Sent</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-gray-500">Failed</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-gray-500">Dead-Letter</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-gray-500">Pending</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {summary.by_day.map(d => (
                  <tr key={d.date} className="hover:bg-gray-50">
                    <td className="px-4 py-2 font-mono text-xs text-gray-600">{d.date}</td>
                    <td className="px-4 py-2 text-right">{d.total}</td>
                    <td className="px-4 py-2 text-right font-mono text-xs">{d.avg_score.toFixed(3)}</td>
                    <td className="px-4 py-2 text-right text-emerald-600">{d.sent}</td>
                    <td className="px-4 py-2 text-right text-red-600">{d.failed}</td>
                    <td className="px-4 py-2 text-right text-orange-600">{d.dead_lettered}</td>
                    <td className="px-4 py-2 text-right text-violet-600">{d.pending}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <p className="text-xs text-gray-400">
            Showing {summary.by_day.length} day(s) · {summary.total} total messages
          </p>
        </>
      )}
    </div>
  )
}
