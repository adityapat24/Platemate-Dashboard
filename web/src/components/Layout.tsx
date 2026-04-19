import { Outlet, NavLink } from 'react-router-dom'

const navItems = [
  { to: '/menu', label: 'Menu', icon: '🍽️' },
  { to: '/history', label: 'History', icon: '📋' },
  { to: '/settings', label: 'Settings', icon: '⚙️' },
]

export default function Layout() {
  return (
    <div className="min-h-screen flex">
      {/* Sidebar */}
      <aside className="w-60 bg-white border-r border-gray-200 flex flex-col shadow-sm">
        <div className="px-6 py-5 border-b border-gray-100">
          <span className="text-xl font-bold text-orange-500 tracking-tight">PlateMate</span>
          <p className="text-xs text-gray-400 mt-0.5">Agentic Dashboard</p>
        </div>
        <nav className="flex-1 py-4 px-3 space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                  isActive
                    ? 'bg-orange-50 text-orange-600'
                    : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
                }`
              }
            >
              <span className="text-base">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="px-6 py-4 border-t border-gray-100">
          <p className="text-xs text-gray-400">Demo Restaurant</p>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
