import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Pipeline } from '../types'
import { useToast } from '../components/Toast'

// ---- empty pipeline skeleton for the create form ---------------------------

const emptyPipeline: Omit<Pipeline, 'created_at' | 'updated_at'> = {
  id: '',
  name: '',
  source_id: '',
  filters: { event_types: [], min_score: 0 },
  destination_ids: [],
  enabled: true,
}

// ---- helpers ---------------------------------------------------------------

function joinIds(ids?: string[]): string {
  return ids?.join(', ') ?? ''
}

function splitIds(raw: string): string[] {
  return raw.split(',').map(s => s.trim()).filter(Boolean)
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
  initial: Omit<Pipeline, 'created_at' | 'updated_at'>
  onSave: (p: Omit<Pipeline, 'created_at' | 'updated_at'>) => void
  onClose: () => void
  isEdit: boolean
}

function PipelineModal({ initial, onSave, onClose, isEdit }: ModalProps) {
  const [form, setForm] = useState(initial)
  // Keep comma-separated strings in local state for controlled inputs
  const [eventTypesRaw, setEventTypesRaw] = useState(joinIds(initial.filters.event_types))
  const [destIdsRaw, setDestIdsRaw] = useState(joinIds(initial.destination_ids))

  function handleSave() {
    onSave({
      ...form,
      filters: {
        ...form.filters,
        event_types: splitIds(eventTypesRaw),
      },
      destination_ids: splitIds(destIdsRaw),
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 space-y-4">
        <h2 className="text-lg font-semibold">{isEdit ? 'Edit Pipeline' : 'New Pipeline'}</h2>

        <div className="grid grid-cols-2 gap-3">
          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">ID</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 disabled:bg-gray-50"
              value={form.id}
              onChange={e => setForm(f => ({ ...f, id: e.target.value }))}
              disabled={isEdit}
              placeholder="adt-to-fhir"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Name</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              placeholder="ADT → FHIR Pipeline"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Source ID (optional — leave blank for any)</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={form.source_id ?? ''}
              onChange={e => setForm(f => ({ ...f, source_id: e.target.value }))}
              placeholder="ward-adt-listener"
            />
          </label>

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Event Types (comma-separated, blank = all)</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={eventTypesRaw}
              onChange={e => setEventTypesRaw(e.target.value)}
              placeholder="ADT^A01, ADT^A03, ADT^A08"
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
              value={form.filters.min_score}
              onChange={e => setForm(f => ({ ...f, filters: { ...f.filters, min_score: Number(e.target.value) } }))}
            />
          </label>

          <div /> {/* spacer */}

          <label className="col-span-2">
            <span className="text-xs font-medium text-gray-500">Destination Channel IDs (comma-separated)</span>
            <input
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              value={destIdsRaw}
              onChange={e => setDestIdsRaw(e.target.value)}
              placeholder="fhir-server-1, fhir-server-2"
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
            onClick={handleSave}
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

export default function Pipelines() {
  const { toast } = useToast()
  const [pipelines, setPipelines] = useState<Pipeline[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modal, setModal] = useState<null | { isEdit: boolean; pipeline: Omit<Pipeline, 'created_at' | 'updated_at'> }>(null)
  const [confirmDelId, setConfirmDelId] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      setPipelines(await api.pipelines.list() ?? [])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  async function handleSave(form: Omit<Pipeline, 'created_at' | 'updated_at'>) {
    try {
      if (modal?.isEdit) {
        await api.pipelines.update(form.id, form)
        toast('Pipeline updated')
      } else {
        await api.pipelines.create(form)
        toast('Pipeline created')
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
      await api.pipelines.delete(id)
      toast(`Pipeline "${id}" deleted`)
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  async function handleToggle(p: Pipeline) {
    try {
      await api.pipelines.update(p.id, { enabled: !p.enabled })
      toast(p.enabled ? 'Pipeline disabled' : 'Pipeline enabled', 'info')
      await load()
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-gray-800">Pipelines</h1>
        <button
          onClick={() => setModal({ isEdit: false, pipeline: { ...emptyPipeline, filters: { event_types: [], min_score: 0 }, destination_ids: [] } })}
          className="px-4 py-2 text-sm rounded-md bg-brand-600 text-white hover:bg-brand-700"
        >
          + New Pipeline
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
                {['ID', 'Name', 'Source', 'Event Types', 'Min Score', 'Destinations', 'Enabled', ''].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {pipelines.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-4 py-8 text-center text-gray-400">
                    No pipelines yet. Create one to route messages from a source to destinations.
                  </td>
                </tr>
              )}
              {pipelines.map(p => (
                <tr key={p.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-mono text-xs text-gray-600">{p.id}</td>
                  <td className="px-4 py-3 font-medium">{p.name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500">
                    {p.source_id || <span className="italic text-gray-400">any</span>}
                  </td>
                  <td className="px-4 py-3 text-gray-500 text-xs max-w-[160px] truncate">
                    {p.filters.event_types?.length
                      ? p.filters.event_types.join(', ')
                      : <span className="italic text-gray-400">all</span>}
                  </td>
                  <td className="px-4 py-3 text-gray-500">{p.filters.min_score.toFixed(2)}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs max-w-[160px] truncate">
                    {p.destination_ids?.length
                      ? p.destination_ids.join(', ')
                      : <span className="italic text-gray-400">none</span>}
                  </td>
                  <td className="px-4 py-3">
                    <Toggle checked={p.enabled} onChange={() => handleToggle(p)} />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2 justify-end">
                      <button
                        onClick={() => setModal({ isEdit: true, pipeline: p })}
                        className="text-xs text-brand-600 hover:underline"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(p.id)}
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
        <PipelineModal
          initial={modal.pipeline}
          isEdit={modal.isEdit}
          onSave={handleSave}
          onClose={() => setModal(null)}
        />
      )}

      {confirmDelId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-xl shadow-xl w-full max-w-sm mx-4 p-6 space-y-4">
            <h2 className="text-base font-semibold text-gray-800">Delete Pipeline</h2>
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
