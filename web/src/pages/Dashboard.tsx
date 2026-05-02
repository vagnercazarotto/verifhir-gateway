import { useEffect, useState } from 'react'
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend,
} from 'recharts'
import { api } from '../api/client'
import type { Message } from '../types'

// ---- helpers ---------------------------------------------------------------

const STATUS_COLORS: Record<string, string> = {
  pending:      '#a78bfa',
  sent:         '#34d399',
  failed:       '#f87171',
  dead_lettered: '#fb923c',
}

function statusBreakdown(msgs: Message[]) {
  const counts: Record<string, number> = {}
  for (const m of msgs) {
    counts[m.status] = (counts[m.status] ?? 0) + 1
  }
  return Object.entries(counts).map(([name, value]) => ({ name, value }))
}

function scoreSparkline(msgs: Message[]) {
  // Group by hour bucket (last 12h), average quality_score per bucket
  const now = Date.now()
  const buckets: Record<number, number[]> = {}
  for (const m of msgs) {
    const age = now - new Date(m.created_at).getTime()
    const h = Math.floor(age / 3_600_000)
    if (h > 11) continue
    const key = 11 - h // 0=oldest, 11=newest
    ;(buckets[key] ??= []).push(m.quality_score)
  }
  return Array.from({ length: 12 }, (_, i) => ({
    h: `-${11 - i}h`,
    score: buckets[i]?.reduce((a, b) => a + b, 0) / (buckets[i]?.length || 1) || null,
  }))
}

function avg(nums: number[]) {
  if (!nums.length) return 0
  return nums.reduce((a, b) => a + b, 0) / nums.length
}

// ---- stat card -------------------------------------------------------------

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
      <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">{label}</p>
      <p className="mt-1 text-3xl font-bold text-gray-900">{value}</p>
      {sub && <p className="mt-1 text-sm text-gray-400">{sub}</p>}
    </div>
  )
}

// ---- page ------------------------------------------------------------------

export default function Dashboard() {
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.messages
      .list({ limit: 1000 })
      .then(setMessages)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-gray-400">
        Loading…
      </div>
    )
  }
  if (error) {
    return (
      <div className="flex items-center justify-center h-full text-red-500">
        {error}
      </div>
    )
  }

  const deadLettered = messages.filter(m => m.status === 'dead_lettered').length
  const avgScore = avg(messages.map(m => m.quality_score))
  const sparkData = scoreSparkline(messages)
  const pieData = statusBreakdown(messages)

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold text-gray-800">Dashboard</h1>

      {/* stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total Messages"    value={messages.length} />
        <StatCard label="Avg Quality Score" value={avgScore.toFixed(3)} />
        <StatCard label="Dead-Letter"       value={deadLettered} sub="last 1 000" />
        <StatCard
          label="Sent"
          value={messages.filter(m => m.status === 'sent').length}
          sub={`${messages.length ? ((messages.filter(m => m.status === 'sent').length / messages.length) * 100).toFixed(0) : 0}% success`}
        />
      </div>

      {/* charts row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* quality sparkline */}
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
          <p className="text-sm font-medium text-gray-600 mb-3">Quality Score / Hour (last 12 h)</p>
          <ResponsiveContainer width="100%" height={180}>
            <LineChart data={sparkData}>
              <XAxis dataKey="h" tick={{ fontSize: 11 }} />
              <YAxis domain={[0, 1]} tick={{ fontSize: 11 }} />
              <Tooltip formatter={(v: number) => v?.toFixed(3)} />
              <Line
                type="monotone"
                dataKey="score"
                stroke="#6366f1"
                strokeWidth={2}
                dot={false}
                connectNulls
              />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* status breakdown */}
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
          <p className="text-sm font-medium text-gray-600 mb-3">Status Breakdown</p>
          {pieData.length === 0 ? (
            <div className="flex items-center justify-center h-[180px] text-gray-300 text-sm">
              No data
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={180}>
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="value"
                  nameKey="name"
                  cx="50%"
                  cy="50%"
                  outerRadius={70}
                  label={({ name, percent }) =>
                    `${name} ${(percent * 100).toFixed(0)}%`
                  }
                >
                  {pieData.map(entry => (
                    <Cell
                      key={entry.name}
                      fill={STATUS_COLORS[entry.name] ?? '#94a3b8'}
                    />
                  ))}
                </Pie>
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>
    </div>
  )
}
