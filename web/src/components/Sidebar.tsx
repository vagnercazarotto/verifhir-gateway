import { NavLink } from 'react-router-dom'

const links = [
  { to: '/dashboard', label: 'Dashboard',  icon: '▦' },
  { to: '/channels',  label: 'Channels',   icon: '⇌' },
  { to: '/messages',  label: 'Messages',   icon: '✉' },
  { to: '/audit',     label: 'Audit Log',  icon: '☰' },
  { to: '/reports',   label: 'Reports',    icon: '⎙' },
]

export default function Sidebar() {
  return (
    <aside className="w-56 shrink-0 flex flex-col bg-brand-700 text-white">
      <div className="px-5 py-5 border-b border-brand-600">
        <span className="text-lg font-bold tracking-tight">VeriFHIR</span>
        <span className="ml-1 text-brand-100 text-sm">Gateway</span>
      </div>

      <nav className="flex-1 py-4 space-y-1 px-2">
        {links.map(({ to, label, icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              [
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors',
                isActive
                  ? 'bg-brand-600 text-white'
                  : 'text-brand-100 hover:bg-brand-600/60 hover:text-white',
              ].join(' ')
            }
          >
            <span className="text-base leading-none">{icon}</span>
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="px-4 py-3 border-t border-brand-600 text-xs text-brand-200">
        v0.1.0
      </div>
    </aside>
  )
}
