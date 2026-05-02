// Reports page — Phase 3.6
// Will show date-range reports with PDF/CSV export.
// Placeholder until the /api/v1/reports endpoint is implemented.

export default function Reports() {
  return (
    <div className="p-6 space-y-4">
      <h1 className="text-xl font-semibold text-gray-800">Reports</h1>

      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-8 text-center space-y-3">
        <p className="text-4xl">⎙</p>
        <p className="font-medium text-gray-700">Reports coming soon</p>
        <p className="text-sm text-gray-500 max-w-sm mx-auto">
          Date-range reports with total messages, average quality score,
          findings per rule, and PDF / CSV export will be available in
          Phase 3.6.
        </p>
      </div>
    </div>
  )
}
