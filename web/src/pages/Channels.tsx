import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Channel, RetryConfig } from '../types'
import { useToast } from '../components/Toast'

// ---- empty channel skeleton for the create form ----------------------------

const emptyRetry: RetryConfig = { max_attempts: 3, initial_backoff_ms: 500, multiplier: 2 }

const emptyChannel: Omit<Channel, 'created_at' | 'updated_at'> = {
  id: '',
  name: '',
  url: '',
  auth_header: '',
  timeout_ms: 10000,
  min_quality_score: 0,
  enabled: true,
  retry: emptyRetry,
}

// ---- toggle ----------------------------------------------------------------

function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className={[
        'relative inline-flex h-5 w-10 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors',
        checked ? 'bg-brand-500' : 'bg-gray-300',
      ].join(' ')}
    >
      <span
        className={[
          'inline-block h-4 w-4 rounded-full bg-white shadow transition-transform',
          checked ? 'translate-x-5' : 'translate-x-0',
        ].join(' ')}
      />
    </button>
  )
}

// ---- modal form ------------------------------------------------------------

interface ModalProps {
  initial: Omit<Channel, 'created_at' | 'updated_at'>
  onSave: (ch: Omit<Channel, 'created_at' | 'updated_at'>) => void
  onClose: () => void
  isEdit: boolean
}

function ChannelModal({ initial, onSave, onClose, isEdit }: ModalProps) {
  const [form, setForm] = useState(initial)

  function field(key: keyof typeof emptyChannel) {
    return (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm(f => ({ ...f, [key]: e.target.value }))
  }

  function retryField(key: keyof RetryConfig) {
    return (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm(f => ({ ...f, retry: { ...f.retry, [key]: Number(e.target.value) } }))
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 space-y-4">
        <h2 className="text-lg font-semibold">{isEdit ? 'Edit Channel' : 'New Channel'}</h2>

        <div className="grid grid-cols-2 gap-3">
          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">ID</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 disabled:bg-gray-50"
              value={form.id}
              onChange={field('id')}
              disabled={isEdit}
              placeholder="my-fhir-server"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Name</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.name}
              onChange={field('name')}
              placeholder="My FHIR Server"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Destination URL</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.url}
              onChange={field('url')}
              placeholder="https://fhir.example.com"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Auth Header (optional)</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.auth_header ?? ''}
              onChange={field('auth_header')}
              placeholder="Bearer token..."
            />
          </label>

          <label>
            <span className="text-xs font-medium text-gray-500">Timeout (ms)</span>
            <input
              type="number"
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.timeout_ms}
              onChange={e => setForm(f => ({ ...f, timeout_ms: Number(e.target.value) }))}
            />
          </label>

          <label>
            <span className="text-xs font-medium text-gray-500">Min Quality Score</span>
            <input
              type="number"
              step="0.01"
              min="0"
              max="1"
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.min_quality_score}
              onChange={e =>
                setForm(f => ({ ...f, min_quality_score: Number(e.target.value) }))
              }
            />
          </label>

          <label>
            <span className="text-xs font-medium text-gray-500">Max Retry Attempts</span>
            <input
              type="number"
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.retry.max_attempts}
              onChange={retryField('max_attempts')}
            />
          </label>

          <label>
            <span className="text-xs font-medium text-gray-500">Initial Backoff (ms)</span>
            <input
              type="number"
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.retry.initial_backoff_ms}
              onChange={retryField('initial_backoff_ms')}
            />
          </label>

          <div className="col-span-2 flex items-center gap-3">
            <Toggle
              checked={form.enabled}
              onChange={v => setForm(f => ({ ...f, enabled: v }))}
            />
            <span className="text-sm text-gray-600">
              {form.enabled ? 'Enabled' : 'Disabled'}
            </span>
          </div>
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm rounded-md border border-gray-300 hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            onClick={() => onSave(form)}
            className="px-4 py-2 text-sm rounded-md bg-brand-600 text-white hover:bg-brand-700"
          >
            {isEdit ? 'Save Changes' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- page ------------------------------------------------------------------

export default function Channels() {
  const { toast } = useToast()
  const [channels, setChannels] = useState<Channel[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modal, setModal] = useState<null | { isEdit: boolean; channel: Omit<Channel, 'created_at' | 'updated_at'> }>(null)
  const [confirmDelId, setConfirmDelId] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      setChannels(await api.channels.list() ?? [])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function handleSave(form: Omit<Channel, 'created_at' | 'updated_at'>) {
    try {
      if (modal?.isEdit) {
        await api.channels.update(form.id, form)
        toast('Channel updated')
      } else {
        await api.channels.create(form)
        toast('Channel created')
      }
      setModal(null)
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  async function handleDelete(id: string) {
    setConfirmDelId(id)
  }

  async function confirmDelete() {
    if (!confirmDelId) return
    const id = confirmDelId
    setConfirmDelId(null)
    try {
      await api.channels.delete(id)
      toast(`Channel "${id}" deleted`)
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  async function handleToggle(ch: Channel) {
    try {
      await api.channels.update(ch.id, { enabled: !ch.enabled })
      toast(ch.enabled ? 'Channel disabled' : 'Channel enabled', 'info')
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-gray-800">Channels</h1>
        <button
          onClick={() => setModal({ isEdit: false, channel: { ...emptyChannel } })}
          className="px-4 py-2 text-sm rounded-md bg-brand-600 text-white hover:bg-brand-700"
        >
          + New Channel
        </button>
      </div>

      {loading && (
        <p className="text-gray-400 text-sm">Loading…</p>
      )}
      {error && (
        <p className="text-red-500 text-sm">{error}</p>
      )}

      {!loading && !error && (
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                {['ID', 'Name', 'URL', 'Min Score', 'Retry', 'Enabled', ''].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {channels.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-gray-400">
                    No channels yet. Create one to get started.
                  </td>
                </tr>
              )}
              {channels.map(ch => (
                <tr key={ch.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-mono text-xs text-gray-600">{ch.id}</td>
                  <td className="px-4 py-3 font-medium">{ch.name}</td>
                  <td className="px-4 py-3 text-gray-500 truncate max-w-[200px]">{ch.url}</td>
                  <td className="px-4 py-3 text-gray-500">{ch.min_quality_score.toFixed(2)}</td>
                  <td className="px-4 py-3 text-gray-500">{ch.retry.max_attempts}×</td>
                  <td className="px-4 py-3">
                    <Toggle checked={ch.enabled} onChange={() => handleToggle(ch)} />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2 justify-end">
                      <button
                        onClick={() => setModal({ isEdit: true, channel: ch })}
                        className="text-xs text-brand-600 hover:underline"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(ch.id)}
                        className="text-xs text-red-500 hover:underline"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal && (
        <ChannelModal
          initial={modal.channel}
          isEdit={modal.isEdit}
          onSave={handleSave}
          onClose={() => setModal(null)}
        />
      )}

      {confirmDelId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-xl shadow-xl w-full max-w-sm mx-4 p-6 space-y-4">
            <h2 className="text-base font-semibold text-gray-800">Delete Channel</h2>
            <p className="text-sm text-gray-600">
              Delete{' '}
              <span className="font-mono font-medium">&ldquo;{confirmDelId}&rdquo;</span>?{' '}
              This cannot be undone.
            </p>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setConfirmDelId(null)}
                className="px-4 py-2 text-sm rounded-md border border-gray-300 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={confirmDelete}
                className="px-4 py-2 text-sm rounded-md bg-red-600 text-white hover:bg-red-700"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
