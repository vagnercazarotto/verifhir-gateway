import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Source, SourceType } from '../types'
import { useToast } from '../components/Toast'

// ---- empty source skeleton for the create form -----------------------------

const emptySource: Omit<Source, 'created_at' | 'updated_at'> = {
  id: '',
  name: '',
  type: 'mllp',
  addr: '',
  enabled: true,
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
  initial: Omit<Source, 'created_at' | 'updated_at'>
  onSave: (src: Omit<Source, 'created_at' | 'updated_at'>) => void
  onClose: () => void
  isEdit: boolean
}

function SourceModal({ initial, onSave, onClose, isEdit }: ModalProps) {
  const [form, setForm] = useState(initial)

  function field(key: keyof typeof emptySource) {
    return (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm(f => ({ ...f, [key]: e.target.value }))
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 space-y-4">
        <h2 className="text-lg font-semibold">{isEdit ? 'Edit Source' : 'New Source'}</h2>

        <div className="grid grid-cols-2 gap-3">
          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">ID</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 disabled:bg-gray-50"
              value={form.id}
              onChange={field('id')}
              disabled={isEdit}
              placeholder="ward-adt-listener"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Name</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.name}
              onChange={field('name')}
              placeholder="Ward ADT Listener"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Type</span>
            <select
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.type}
              onChange={e => setForm(f => ({ ...f, type: e.target.value as SourceType }))}
            >
              <option value="mllp">MLLP — TCP listener</option>
            </select>
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Listen Address (host:port)</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.addr}
              onChange={field('addr')}
              placeholder="0.0.0.0:2575"
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

export default function Sources() {
  const { toast } = useToast()
  const [sources, setSources] = useState<Source[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modal, setModal] = useState<null | { isEdit: boolean; source: Omit<Source, 'created_at' | 'updated_at'> }>(null)
  const [confirmDelId, setConfirmDelId] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      setSources(await api.sources.list() ?? [])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function handleSave(form: Omit<Source, 'created_at' | 'updated_at'>) {
    try {
      if (modal?.isEdit) {
        await api.sources.update(form.id, form)
        toast('Source updated')
      } else {
        await api.sources.create(form)
        toast('Source created')
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
      await api.sources.delete(id)
      toast(`Source "${id}" deleted`)
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  async function handleToggle(src: Source) {
    try {
      await api.sources.update(src.id, { enabled: !src.enabled })
      toast(src.enabled ? 'Source disabled' : 'Source enabled', 'info')
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-gray-800">Sources</h1>
        <button
          onClick={() => setModal({ isEdit: false, source: { ...emptySource } })}
          className="px-4 py-2 text-sm rounded-md bg-brand-600 text-white hover:bg-brand-700"
        >
          + New Source
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
                {['ID', 'Name', 'Type', 'Address', 'Enabled', ''].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {sources.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-gray-400">
                    No sources yet. Create one to get started.
                  </td>
                </tr>
              )}
              {sources.map(src => (
                <tr key={src.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-mono text-xs text-gray-600">{src.id}</td>
                  <td className="px-4 py-3 font-medium">{src.name}</td>
                  <td className="px-4 py-3">
                    <span className="inline-flex px-2 py-0.5 rounded-full text-xs font-medium bg-teal-100 text-teal-700">
                      {src.type.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500">{src.addr}</td>
                  <td className="px-4 py-3">
                    <Toggle checked={src.enabled} onChange={() => handleToggle(src)} />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2 justify-end">
                      <button
                        onClick={() => setModal({ isEdit: true, source: src })}
                        className="text-xs text-brand-600 hover:underline"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(src.id)}
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
        <SourceModal
          initial={modal.source}
          isEdit={modal.isEdit}
          onSave={handleSave}
          onClose={() => setModal(null)}
        />
      )}

      {confirmDelId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-xl shadow-xl w-full max-w-sm mx-4 p-6 space-y-4">
            <h2 className="text-base font-semibold text-gray-800">Delete Source</h2>
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
