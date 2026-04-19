import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import type { Dish } from '../api/types'
import * as dishesApi from '../api/dishes'
import ActionModal from '../components/ActionModal'
import StatusBadge from '../components/StatusBadge'

type ActionType = 'REMOVE_DISH' | 'RESTORE_DISH' | 'CHANGE_PRICE' | 'RUN_PROMO'

export default function MenuView() {
  const navigate = useNavigate()
  const [dishes, setDishes] = useState<Dish[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [error, setError] = useState('')
  const [toast, setToast] = useState('')
  const [search, setSearch] = useState('')
  const [selectedDish, setSelectedDish] = useState<Dish | null>(null)
  const [actionType, setActionType] = useState<ActionType | null>(null)

  const loadDishes = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await dishesApi.listDishes()
      setDishes(data)
    } catch {
      setError('Failed to load dishes')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadDishes() }, [loadDishes])

  async function handleSync() {
    setSyncing(true)
    try {
      const res = await dishesApi.syncMenu()
      showToast(`Synced ${res.synced} items from Uber Eats`)
      await loadDishes()
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      showToast(msg || 'Sync failed — check Uber Eats connection', true)
    } finally {
      setSyncing(false)
    }
  }

  function showToast(msg: string, isError = false) {
    setToast(isError ? `❌ ${msg}` : `✅ ${msg}`)
    setTimeout(() => setToast(''), 4000)
  }

  function openAction(dish: Dish, type: ActionType) {
    setSelectedDish(dish)
    setActionType(type)
  }

  function closeModal() {
    setSelectedDish(null)
    setActionType(null)
  }

  function handleActionSuccess(message: string) {
    closeModal()
    showToast(message)
    loadDishes()
  }

  const filtered = dishes.filter(d =>
    d.canonical_name.toLowerCase().includes(search.toLowerCase()) ||
    d.category.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div className="p-8">
      {/* Toast */}
      {toast && (
        <div className="fixed top-6 right-6 z-50 bg-white border border-gray-200 rounded-xl shadow-lg px-5 py-3 text-sm font-medium text-gray-800 max-w-sm">
          {toast}
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Unified Menu</h1>
          <p className="text-sm text-gray-500 mt-0.5">{dishes.length} dishes across all platforms</p>
        </div>
        <button
          onClick={handleSync}
          disabled={syncing}
          className="flex items-center gap-2 px-4 py-2 bg-orange-500 hover:bg-orange-600 text-white text-sm font-semibold rounded-lg transition-colors disabled:opacity-50"
        >
          {syncing ? (
            <span className="animate-spin">⟳</span>
          ) : (
            <span>↻</span>
          )}
          {syncing ? 'Syncing...' : 'Sync Menu'}
        </button>
      </div>

      {/* Search */}
      <div className="relative mb-5">
        <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">🔍</span>
        <input
          type="text"
          placeholder="Search dishes..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="w-full pl-9 pr-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-orange-400 bg-white"
        />
      </div>

      {/* Error */}
      {error && (
        <div className="mb-4 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {/* Empty state */}
      {!loading && filtered.length === 0 && !error && (
        <div className="text-center py-20">
          <p className="text-4xl mb-3">🍽️</p>
          <p className="text-gray-600 font-medium">No dishes yet</p>
          <p className="text-sm text-gray-400 mt-1 mb-4">Connect Uber Eats and sync your menu to get started</p>
          <button
            onClick={handleSync}
            className="px-5 py-2 bg-orange-500 hover:bg-orange-600 text-white text-sm font-semibold rounded-lg"
          >
            Sync Menu Now
          </button>
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div className="text-center py-16 text-gray-400 text-sm">Loading dishes...</div>
      )}

      {/* Table */}
      {!loading && filtered.length > 0 && (
        <div className="bg-white rounded-2xl border border-gray-200 overflow-hidden shadow-sm">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50">
                <th className="text-left px-5 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider">Dish</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider">Category</th>
                <th className="text-right px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider">Price</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                <th className="text-right px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider">Health</th>
                <th className="px-5 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((dish, i) => {
                const ue = dish.platforms.uber_eats
                const isSuspended = ue?.suspended ?? false
                const priceCents = ue?.price_cents ?? 0
                const health = dish.analytics.health_score

                return (
                  <tr
                    key={dish.id}
                    className={`border-b border-gray-50 hover:bg-orange-50/40 transition-colors cursor-pointer ${i % 2 === 0 ? '' : 'bg-gray-50/30'}`}
                    onClick={() => navigate(`/menu/${dish.id}`)}
                  >
                    <td className="px-5 py-4">
                      <div className="font-medium text-gray-900">{dish.canonical_name}</div>
                      {ue && (
                        <div className="text-xs text-gray-400 mt-0.5">ID: {ue.item_id}</div>
                      )}
                    </td>
                    <td className="px-4 py-4 text-gray-500">{dish.category || '—'}</td>
                    <td className="px-4 py-4 text-right font-medium text-gray-800">
                      {priceCents > 0 ? `$${(priceCents / 100).toFixed(2)}` : '—'}
                    </td>
                    <td className="px-4 py-4">
                      {ue ? (
                        <StatusBadge status={isSuspended ? 'suspended' : 'active'} size="sm" />
                      ) : (
                        <span className="text-gray-400 text-xs">No UE data</span>
                      )}
                    </td>
                    <td className="px-4 py-4 text-right">
                      {health > 0 ? (
                        <span className={`font-semibold ${health >= 7 ? 'text-green-600' : health >= 5 ? 'text-yellow-600' : 'text-red-500'}`}>
                          {health.toFixed(1)}
                        </span>
                      ) : (
                        <span className="text-gray-300">—</span>
                      )}
                    </td>
                    <td className="px-5 py-4" onClick={e => e.stopPropagation()}>
                      <div className="flex items-center gap-1 justify-end">
                        {ue && (
                          <>
                            {isSuspended ? (
                              <button
                                onClick={() => openAction(dish, 'RESTORE_DISH')}
                                className="px-2.5 py-1 text-xs font-medium rounded-md bg-green-100 text-green-700 hover:bg-green-200 transition-colors"
                              >
                                Restore
                              </button>
                            ) : (
                              <button
                                onClick={() => openAction(dish, 'REMOVE_DISH')}
                                className="px-2.5 py-1 text-xs font-medium rounded-md bg-red-100 text-red-700 hover:bg-red-200 transition-colors"
                              >
                                86
                              </button>
                            )}
                            <button
                              onClick={() => openAction(dish, 'CHANGE_PRICE')}
                              className="px-2.5 py-1 text-xs font-medium rounded-md bg-blue-100 text-blue-700 hover:bg-blue-200 transition-colors"
                            >
                              Price
                            </button>
                            <button
                              onClick={() => openAction(dish, 'RUN_PROMO')}
                              className="px-2.5 py-1 text-xs font-medium rounded-md bg-orange-100 text-orange-700 hover:bg-orange-200 transition-colors"
                            >
                              Promo
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Action modal */}
      {selectedDish && actionType && (
        <ActionModal
          dish={selectedDish}
          actionType={actionType}
          onClose={closeModal}
          onSuccess={handleActionSuccess}
        />
      )}
    </div>
  )
}
