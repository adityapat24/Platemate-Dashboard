import { useState, useEffect } from 'react'
import type { Action } from '../api/types'
import * as actionsApi from '../api/actions'
import StatusBadge from '../components/StatusBadge'

const ACTION_TYPE_LABELS: Record<string, string> = {
  REMOVE_DISH: 'Remove Dish',
  RESTORE_DISH: 'Restore Dish',
  CHANGE_PRICE: 'Change Price',
  RUN_PROMO: 'Run Promo',
  ROLLBACK: 'Rollback',
}

export default function ActionHistory() {
  const [actions, setActions] = useState<Action[]>([])
  const [loading, setLoading] = useState(true)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [rolling, setRolling] = useState<string | null>(null)
  const [toast, setToast] = useState('')
  const [filterType, setFilterType] = useState('')

  useEffect(() => {
    load()
  }, [filterType])

  async function load() {
    setLoading(true)
    try {
      const data = await actionsApi.listActions(filterType ? { action_type: filterType } : {})
      setActions(data)
    } finally {
      setLoading(false)
    }
  }

  async function handleRollback(actionId: string) {
    setRolling(actionId)
    try {
      const res = await actionsApi.rollbackAction(actionId)
      showToast(`✅ Rollback: ${res.message}`)
      await load()
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      showToast(`❌ ${msg || 'Rollback failed'}`)
    } finally {
      setRolling(null)
    }
  }

  function showToast(msg: string) {
    setToast(msg)
    setTimeout(() => setToast(''), 4000)
  }

  return (
    <div className="p-8">
      {/* Toast */}
      {toast && (
        <div className="fixed top-6 right-6 z-50 bg-white border border-gray-200 rounded-xl shadow-lg px-5 py-3 text-sm font-medium text-gray-800">
          {toast}
        </div>
      )}

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Action History</h1>
          <p className="text-sm text-gray-500 mt-0.5">Audit trail of all executed actions</p>
        </div>
        <select
          value={filterType}
          onChange={e => setFilterType(e.target.value)}
          className="px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-400"
        >
          <option value="">All Types</option>
          {Object.entries(ACTION_TYPE_LABELS).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      </div>

      {loading && <div className="text-center py-16 text-gray-400 text-sm">Loading...</div>}

      {!loading && actions.length === 0 && (
        <div className="text-center py-20">
          <p className="text-4xl mb-3">📋</p>
          <p className="text-gray-600 font-medium">No actions yet</p>
          <p className="text-sm text-gray-400 mt-1">Actions will appear here after you execute them from the menu view</p>
        </div>
      )}

      {!loading && actions.length > 0 && (
        <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
          {actions.map((action, i) => (
            <div key={action.id} className={`border-b border-gray-50 last:border-0 ${i % 2 === 0 ? '' : 'bg-gray-50/30'}`}>
              <div
                className="flex items-center px-5 py-4 cursor-pointer hover:bg-orange-50/30 transition-colors"
                onClick={() => setExpanded(expanded === action.id ? null : action.id)}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-gray-900 text-sm">
                      {ACTION_TYPE_LABELS[action.action_type] ?? action.action_type}
                    </span>
                    {action.dish_name && (
                      <span className="text-gray-400 text-sm">— {action.dish_name}</span>
                    )}
                    {action.rolled_back && (
                      <span className="text-xs bg-gray-100 text-gray-500 px-2 py-0.5 rounded-full">Rolled back</span>
                    )}
                  </div>
                  <p className="text-xs text-gray-400 mt-0.5">
                    {new Date(action.created_at).toLocaleString()}
                  </p>
                </div>
                <div className="flex items-center gap-3 ml-4">
                  <StatusBadge status={action.status} size="sm" />
                  {action.can_rollback && !action.rolled_back && (
                    <button
                      onClick={e => { e.stopPropagation(); handleRollback(action.id) }}
                      disabled={rolling === action.id}
                      className="px-3 py-1 text-xs font-medium rounded-md border border-gray-200 text-gray-600 hover:bg-gray-100 disabled:opacity-50"
                    >
                      {rolling === action.id ? 'Rolling back...' : 'Undo'}
                    </button>
                  )}
                  <span className="text-gray-300 text-xs">{expanded === action.id ? '▲' : '▼'}</span>
                </div>
              </div>

              {/* Expanded detail */}
              {expanded === action.id && (
                <div className="px-5 pb-4 pt-0 border-t border-gray-50">
                  <div className="grid grid-cols-2 gap-4 mt-3">
                    <div>
                      <p className="text-xs font-semibold text-gray-400 uppercase mb-2">Platform Results</p>
                      {action.results.map(r => (
                        <div key={r.platform} className="flex items-center gap-2 py-1.5">
                          <StatusBadge status={r.status} size="sm" />
                          <span className="text-sm text-gray-700 capitalize">{r.platform.replace('_', ' ')}</span>
                          {r.message && <span className="text-xs text-gray-400 ml-1">— {r.message}</span>}
                        </div>
                      ))}
                      {action.results.length === 0 && (
                        <p className="text-xs text-gray-400">No platform results</p>
                      )}
                    </div>
                    <div>
                      <p className="text-xs font-semibold text-gray-400 uppercase mb-2">Details</p>
                      <div className="space-y-1 text-xs text-gray-500">
                        <div>ID: <span className="font-mono text-gray-600">{action.id}</span></div>
                        <div>By: {action.initiator}</div>
                        {Object.entries(action.params || {}).map(([k, v]) => (
                          <div key={k}>{k}: <span className="text-gray-700">{String(v)}</span></div>
                        ))}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
