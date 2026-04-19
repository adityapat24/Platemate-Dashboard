import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import type { Dish } from '../api/types'
import * as dishesApi from '../api/dishes'
import ActionModal from '../components/ActionModal'
import StatusBadge from '../components/StatusBadge'

type ActionType = 'REMOVE_DISH' | 'RESTORE_DISH' | 'CHANGE_PRICE' | 'RUN_PROMO'

export default function DishDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [dish, setDish] = useState<Dish | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionType, setActionType] = useState<ActionType | null>(null)
  const [toast, setToast] = useState('')

  useEffect(() => {
    if (!id) return
    dishesApi.getDish(id)
      .then(setDish)
      .catch(() => setError('Dish not found'))
      .finally(() => setLoading(false))
  }, [id])

  function showToast(msg: string) {
    setToast(msg)
    setTimeout(() => setToast(''), 4000)
  }

  function handleActionSuccess(message: string) {
    setActionType(null)
    showToast(`✅ ${message}`)
    // Reload dish data
    if (id) dishesApi.getDish(id).then(setDish).catch(() => {})
  }

  if (loading) return <div className="p-8 text-gray-400 text-sm">Loading...</div>
  if (error || !dish) return (
    <div className="p-8">
      <button onClick={() => navigate('/menu')} className="text-sm text-orange-500 hover:underline mb-4 block">← Back to menu</button>
      <p className="text-gray-600">{error || 'Dish not found'}</p>
    </div>
  )

  const ue = dish.platforms.uber_eats
  const a = dish.analytics
  const f = dish.financials

  const statCards = [
    { label: 'Overall', value: a.avg_overall?.toFixed(1) || '—', sub: `${a.platemate_review_count} reviews` },
    { label: 'Taste', value: a.avg_taste?.toFixed(1) || '—' },
    { label: 'Portion', value: a.avg_portion?.toFixed(1) || '—' },
    { label: 'Value', value: a.avg_value?.toFixed(1) || '—' },
    { label: 'Reorder', value: a.reorder_rate > 0 ? `${Math.round(a.reorder_rate * 100)}%` : '—' },
    { label: 'Health', value: a.health_score > 0 ? a.health_score.toFixed(1) : '—' },
  ]

  return (
    <div className="p-8 max-w-4xl">
      {/* Toast */}
      {toast && (
        <div className="fixed top-6 right-6 z-50 bg-white border border-gray-200 rounded-xl shadow-lg px-5 py-3 text-sm font-medium text-gray-800">
          {toast}
        </div>
      )}

      {/* Breadcrumb */}
      <button onClick={() => navigate('/menu')} className="text-sm text-orange-500 hover:underline mb-5 block">
        ← Back to menu
      </button>

      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{dish.canonical_name}</h1>
          <p className="text-sm text-gray-500 mt-1">{dish.category}</p>
        </div>
        {ue && (
          <div className="flex gap-2">
            {ue.suspended ? (
              <button onClick={() => setActionType('RESTORE_DISH')}
                className="px-4 py-2 bg-green-500 hover:bg-green-600 text-white text-sm font-semibold rounded-lg">
                Restore Dish
              </button>
            ) : (
              <button onClick={() => setActionType('REMOVE_DISH')}
                className="px-4 py-2 bg-red-500 hover:bg-red-600 text-white text-sm font-semibold rounded-lg">
                Remove (86)
              </button>
            )}
            <button onClick={() => setActionType('CHANGE_PRICE')}
              className="px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white text-sm font-semibold rounded-lg">
              Change Price
            </button>
            <button onClick={() => setActionType('RUN_PROMO')}
              className="px-4 py-2 bg-orange-500 hover:bg-orange-600 text-white text-sm font-semibold rounded-lg">
              Run Promo
            </button>
          </div>
        )}
      </div>

      <div className="grid grid-cols-3 gap-5">
        {/* Analytics */}
        <div className="col-span-2 bg-white rounded-2xl border border-gray-200 p-5 shadow-sm">
          <h2 className="text-sm font-semibold text-gray-700 mb-4">Performance Metrics</h2>
          <div className="grid grid-cols-3 gap-3">
            {statCards.map(stat => (
              <div key={stat.label} className="bg-gray-50 rounded-xl p-3">
                <p className="text-xs text-gray-500 mb-1">{stat.label}</p>
                <p className="text-xl font-bold text-gray-900">{stat.value}</p>
                {stat.sub && <p className="text-xs text-gray-400 mt-0.5">{stat.sub}</p>}
              </div>
            ))}
          </div>
          {a.trend_direction && (
            <div className="mt-4 px-3 py-2 bg-blue-50 rounded-lg">
              <p className="text-xs text-blue-700">
                Trend: <span className="font-semibold capitalize">{a.trend_direction}</span>
              </p>
            </div>
          )}
        </div>

        {/* Right column */}
        <div className="space-y-4">
          {/* Uber Eats */}
          <div className="bg-white rounded-2xl border border-gray-200 p-4 shadow-sm">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-gray-700">Uber Eats</h3>
              {ue ? (
                <StatusBadge status={ue.suspended ? 'suspended' : 'active'} size="sm" />
              ) : (
                <StatusBadge status="not_connected" size="sm" />
              )}
            </div>
            {ue ? (
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-gray-500">Price</span>
                  <span className="font-semibold">${(ue.price_cents / 100).toFixed(2)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Item ID</span>
                  <span className="font-mono text-xs text-gray-600">{ue.item_id}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Last Synced</span>
                  <span className="text-xs text-gray-400">
                    {ue.last_synced ? new Date(ue.last_synced).toLocaleDateString() : '—'}
                  </span>
                </div>
              </div>
            ) : (
              <p className="text-xs text-gray-400">Not connected to Uber Eats</p>
            )}
          </div>

          {/* Financials */}
          {(f.food_cost > 0 || f.weekly_units > 0) && (
            <div className="bg-white rounded-2xl border border-gray-200 p-4 shadow-sm">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Financials</h3>
              <div className="space-y-2 text-sm">
                {f.food_cost > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">Food Cost</span>
                    <span className="font-semibold">${f.food_cost.toFixed(2)}</span>
                  </div>
                )}
                {f.margin_percent > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">Margin</span>
                    <span className="font-semibold">{Math.round(f.margin_percent * 100)}%</span>
                  </div>
                )}
                {f.weekly_units > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">Units/week</span>
                    <span className="font-semibold">{f.weekly_units}</span>
                  </div>
                )}
                {f.weekly_revenue > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">Revenue/week</span>
                    <span className="font-semibold">${f.weekly_revenue.toFixed(0)}</span>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Action modal */}
      {actionType && (
        <ActionModal
          dish={dish}
          actionType={actionType}
          onClose={() => setActionType(null)}
          onSuccess={handleActionSuccess}
        />
      )}
    </div>
  )
}
