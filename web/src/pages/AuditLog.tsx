// Audit Log Viewer
// The gateway writes JSON audit entries to .local/audit/*.jsonl.
// Until the REST API exposes an /api/v1/audit endpoint this page
// shows a placeholder explaining how to access the raw logs.
//
// Phase 3 deliverable: the page structure + placeholder is ready;
// the backend endpoint will be wired in a follow-up.

export default function AuditLog() {
  return (
    <div className="p-6 space-y-4">
      <h1 className="text-xl font-semibold text-gray-800">Audit Log</h1>

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center space-y-3">
        <p className="text-4xl">☰</p>
        <p className="font-medium text-gray-700">Audit log viewer coming soon</p>
        <p className="text-sm text-gray-500 max-w-sm mx-auto">
          The gateway writes a 5-stage JSON audit trail to{' '}
          <code className="bg-gray-100 px-1 rounded text-xs">.local/audit/</code>.
          A searchable viewer with per-message detail and PDF export will be
          available once the <code className="bg-gray-100 px-1 rounded text-xs">/api/v1/audit</code>{' '}
          endpoint is implemented in Phase 3.5.
        </p>
      </div>
    </div>
  )
}
