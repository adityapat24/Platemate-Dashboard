import { useState } from 'react'
import type { Dish } from '../api/types'
import * as actionsApi from '../api/actions'

type ActionType = 'REMOVE_DISH' | 'RESTORE_DISH' | 'CHANGE_PRICE' | 'RUN_PROMO'

interface Props {
  dish: Dish
  actionType: ActionType
  onClose: () => void
  onSuccess: (message: string) => void
}

export default function ActionModal({ dish, actionType, onClose, onSuccess }: Props) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // CHANGE_PRICE fields
  const [newPriceDollars, setNewPriceDollars] = useState(
    dish.platforms.uber_eats
      ? (dish.platforms.uber_eats.price_cents / 100).toFixed(2)
      : '0.00'
  )

  // RUN_PROMO fields
  const [promoType, setPromoType] = useState('MENU_ITEM_DISCOUNT')
  const [discountPercent, setDiscountPercent] = useState(20)
  const [userGroup, setUserGroup] = useState('ALL_CUSTOMERS')
  const [durationDays, setDurationDays] = useState(7)

  const currentPriceCents = dish.platforms.uber_eats?.price_cents ?? 0
  const isSuspended = dish.platforms.uber_eats?.suspended ?? false

  async function handleExecute() {
    setLoading(true)
    setError('')
    try {
      let result
      switch (actionType) {
        case 'REMOVE_DISH':
          result = await actionsApi.removeDish(dish.id)
          break
        case 'RESTORE_DISH':
          result = await actionsApi.restoreDish(dish.id)
          break
        case 'CHANGE_PRICE': {
          const cents = Math.round(parseFloat(newPriceDollars) * 100)
          if (isNaN(cents) || cents <= 0) throw new Error('Enter a valid price')
          result = await actionsApi.changePrice(dish.id, cents)
          break
        }
        case 'RUN_PROMO': {
          const now = new Date()
          const end = new Date(now.getTime() + durationDays * 24 * 60 * 60 * 1000)
          result = await actionsApi.runPromo(dish.id, {
            promo_type: promoType,
            discount_percent: discountPercent,
            user_group: userGroup,
            unlimited_budget: true,
            external_promo_id: `pm_${dish.id}_${Date.now()}`,
          })
          break
        }
      }
      onSuccess(result.result.message || `${actionType} executed successfully`)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Action failed'
      // Try to extract API error message
      const apiMsg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      setError(apiMsg || msg)
    } finally {
      setLoading(false)
    }
  }

  const titles: Record<ActionType, string> = {
    REMOVE_DISH: 'Remove Dish',
    RESTORE_DISH: 'Restore Dish',
    CHANGE_PRICE: 'Change Price',
    RUN_PROMO: 'Run Promotion',
  }
  const descriptions: Record<ActionType, string> = {
    REMOVE_DISH: `This will suspend "${dish.canonical_name}" on Uber Eats, marking it as sold out.`,
    RESTORE_DISH: `This will restore "${dish.canonical_name}" on Uber Eats, making it available again.`,
    CHANGE_PRICE: `Update the Uber Eats price for "${dish.canonical_name}".`,
    RUN_PROMO: `Create a promotion for "${dish.canonical_name}" on Uber Eats.`,
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-gray-100 flex items-start justify-between">
          <div>
            <h2 className="text-base font-semibold text-gray-900">{titles[actionType]}</h2>
            <p className="text-sm text-gray-500 mt-0.5">{dish.canonical_name}</p>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none mt-0.5">✕</button>
        </div>

        {/* Body */}
        <div className="px-6 py-5 space-y-4">
          <p className="text-sm text-gray-600">{descriptions[actionType]}</p>

          {/* CHANGE_PRICE inputs */}
          {actionType === 'CHANGE_PRICE' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">New Price (USD)</label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  type="number"
                  step="0.01"
                  min="0.01"
                  value={newPriceDollars}
                  onChange={e => setNewPriceDollars(e.target.value)}
                  className="w-full pl-7 pr-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-400"
                />
              </div>
              <p className="text-xs text-gray-400 mt-1">
                Current: ${(currentPriceCents / 100).toFixed(2)}
              </p>
            </div>
          )}

          {/* RUN_PROMO inputs */}
          {actionType === 'RUN_PROMO' && (
            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Promotion Type</label>
                <select
                  value={promoType}
                  onChange={e => setPromoType(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-400"
                >
                  <option value="MENU_ITEM_DISCOUNT">% Off This Item</option>
                  <option value="PERCENTOFF">% Off Entire Order</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Discount (%)</label>
                <input
                  type="number"
                  min="1"
                  max="90"
                  value={discountPercent}
                  onChange={e => setDiscountPercent(parseInt(e.target.value))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-400"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Audience</label>
                <select
                  value={userGroup}
                  onChange={e => setUserGroup(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-400"
                >
                  <option value="ALL_CUSTOMERS">All Customers</option>
                  <option value="FIRST_TIME_CUSTOMERS">New Customers Only</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Duration (days)</label>
                <input
                  type="number"
                  min="1"
                  max="90"
                  value={durationDays}
                  onChange={e => setDurationDays(parseInt(e.target.value))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-400"
                />
              </div>
            </div>
          )}

          {/* Platform impact */}
          <div className="rounded-lg bg-gray-50 px-4 py-3">
            <p className="text-xs font-medium text-gray-500 mb-2">PLATFORM IMPACT</p>
            <div className="flex items-center gap-2">
              <span className="text-sm">🟠 Uber Eats</span>
              <span className="text-xs text-gray-500">— Automated</span>
            </div>
          </div>

          {error && (
            <div className="rounded-lg bg-red-50 border border-red-200 px-4 py-3">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-gray-100 flex items-center justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleExecute}
            disabled={loading}
            className={`px-5 py-2 text-sm font-semibold rounded-lg transition-colors ${
              actionType === 'REMOVE_DISH'
                ? 'bg-red-500 hover:bg-red-600 text-white'
                : 'bg-orange-500 hover:bg-orange-600 text-white'
            } disabled:opacity-50 disabled:cursor-not-allowed`}
          >
            {loading ? 'Executing...' : 'Execute'}
          </button>
        </div>
      </div>
    </div>
  )
}
