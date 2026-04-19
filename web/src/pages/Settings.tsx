import { useState, useEffect, useCallback } from 'react'
import type { PlatformConnection, UberEatsStore } from '../api/types'
import * as onboardingApi from '../api/onboarding'
import StatusBadge from '../components/StatusBadge'

const PLATFORM_META: Record<string, { name: string; icon: string; description: string }> = {
  uber_eats: { name: 'Uber Eats', icon: '🟠', description: 'Delivery menu management, promotions, store status' },
  toast: { name: 'Toast POS', icon: '🍞', description: 'In-house sales data, stock management' },
  r365: { name: 'Restaurant365', icon: '📊', description: 'Food cost data, ingredient costing' },
  google: { name: 'Google Reviews', icon: '⭐', description: 'Review data, dish mention analysis' },
}

export default function Settings() {
  const [connections, setConnections] = useState<PlatformConnection[]>([])
  const [loading, setLoading] = useState(true)
  const [toast, setToast] = useState('')
  const [toastError, setToastError] = useState(false)

  // Uber Eats onboarding state
  const [ueStep, setUeStep] = useState<'idle' | 'selecting' | 'provisioning' | 'done'>('idle')
  const [ueStores, setUeStores] = useState<UberEatsStore[]>([])
  const [selectedStores, setSelectedStores] = useState<Set<string>>(new Set())
  const [provisioning, setProvisioning] = useState(false)
  const [loadingStores, setLoadingStores] = useState(false)

  const loadConnections = useCallback(async () => {
    try {
      const data = await onboardingApi.getAllConnectionStatuses()
      setConnections(data)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadConnections() }, [loadConnections])

  function showToast(msg: string, error = false) {
    setToast(msg)
    setToastError(error)
    setTimeout(() => setToast(''), 4000)
  }

  async function handleConnectUberEats() {
    try {
      const { auth_url } = await onboardingApi.getUberAuthURL()
      window.location.href = auth_url
    } catch {
      showToast('Failed to get Uber Eats auth URL', true)
    }
  }

  async function handleLoadStores() {
    setLoadingStores(true)
    try {
      const stores = await onboardingApi.getUberStores()
      setUeStores(stores)
      setUeStep('selecting')
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      showToast(msg || 'Failed to load stores — complete OAuth first', true)
    } finally {
      setLoadingStores(false)
    }
  }

  async function handleProvision() {
    if (selectedStores.size === 0) {
      showToast('Select at least one store', true)
      return
    }
    setProvisioning(true)
    try {
      const res = await onboardingApi.provisionUberStores([...selectedStores])
      showToast(`Connected ${res.provisioned.length} store(s) to Uber Eats`)
      setUeStep('done')
      await loadConnections()
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      showToast(msg || 'Provisioning failed', true)
    } finally {
      setProvisioning(false)
    }
  }

  function toggleStore(storeId: string) {
    setSelectedStores(prev => {
      const next = new Set(prev)
      next.has(storeId) ? next.delete(storeId) : next.add(storeId)
      return next
    })
  }

  const uberConn = connections.find(c => c.platform === 'uber_eats')
  const isUberConnected = uberConn?.status === 'connected'

  return (
    <div className="p-8 max-w-2xl">
      {/* Toast */}
      {toast && (
        <div className={`fixed top-6 right-6 z-50 rounded-xl shadow-lg px-5 py-3 text-sm font-medium border ${
          toastError ? 'bg-red-50 border-red-200 text-red-700' : 'bg-white border-gray-200 text-gray-800'
        }`}>
          {toast}
        </div>
      )}

      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="text-sm text-gray-500 mt-0.5">Manage platform connections and integrations</p>
      </div>

      {/* Platform connections */}
      <div>
        <h2 className="text-base font-semibold text-gray-700 mb-3">Platform Connections</h2>
        <div className="space-y-3">
          {loading ? (
            <div className="text-sm text-gray-400">Loading connections...</div>
          ) : (
            Object.entries(PLATFORM_META).map(([platform, meta]) => {
              const conn = connections.find(c => c.platform === platform)
              const status = conn?.status ?? 'disconnected'

              return (
                <div key={platform} className="bg-white rounded-2xl border border-gray-200 p-4 shadow-sm">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="text-2xl">{meta.icon}</span>
                      <div>
                        <p className="font-semibold text-gray-900 text-sm">{meta.name}</p>
                        <p className="text-xs text-gray-400 mt-0.5">{meta.description}</p>
                        {conn?.store_name && (
                          <p className="text-xs text-green-600 mt-0.5">📍 {conn.store_name}</p>
                        )}
                        {conn?.last_sync && status === 'connected' && (
                          <p className="text-xs text-gray-400 mt-0.5">
                            Last sync: {new Date(conn.last_sync).toLocaleString()}
                          </p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <StatusBadge status={status} />
                      {platform === 'uber_eats' && status !== 'connected' && (
                        <button
                          onClick={handleConnectUberEats}
                          className="px-4 py-1.5 bg-black text-white text-xs font-semibold rounded-lg hover:bg-gray-800 transition-colors"
                        >
                          Connect
                        </button>
                      )}
                      {platform !== 'uber_eats' && status !== 'connected' && (
                        <span className="text-xs text-gray-400 bg-gray-100 px-3 py-1.5 rounded-lg">Coming soon</span>
                      )}
                    </div>
                  </div>
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* Uber Eats provisioning flow (shown after OAuth callback) */}
      {!isUberConnected && (
        <div className="mt-6 bg-orange-50 border border-orange-200 rounded-2xl p-5">
          <h3 className="text-sm font-semibold text-orange-800 mb-2">Complete Uber Eats Setup</h3>
          <p className="text-xs text-orange-700 mb-4">
            After authorizing PlateMate in Uber Eats, load your stores below to complete setup.
          </p>

          {ueStep === 'idle' && (
            <button
              onClick={handleLoadStores}
              disabled={loadingStores}
              className="px-4 py-2 bg-orange-500 hover:bg-orange-600 text-white text-sm font-semibold rounded-lg disabled:opacity-50"
            >
              {loadingStores ? 'Loading stores...' : 'Load My Stores'}
            </button>
          )}

          {ueStep === 'selecting' && (
            <div>
              <p className="text-xs text-orange-700 mb-3">Select the stores to connect:</p>
              <div className="space-y-2 mb-4">
                {ueStores.length === 0 && (
                  <p className="text-xs text-gray-500">No stores found. Make sure you've authorized PlateMate.</p>
                )}
                {ueStores.map(store => (
                  <label key={store.id} className="flex items-center gap-3 bg-white rounded-lg px-4 py-3 cursor-pointer border border-orange-100 hover:border-orange-300">
                    <input
                      type="checkbox"
                      checked={selectedStores.has(store.id)}
                      onChange={() => toggleStore(store.id)}
                      className="w-4 h-4 accent-orange-500"
                    />
                    <div>
                      <p className="text-sm font-medium text-gray-900">{store.name}</p>
                      <p className="text-xs text-gray-400">{store.id}</p>
                    </div>
                  </label>
                ))}
              </div>
              <button
                onClick={handleProvision}
                disabled={provisioning || selectedStores.size === 0}
                className="px-5 py-2 bg-orange-500 hover:bg-orange-600 text-white text-sm font-semibold rounded-lg disabled:opacity-50"
              >
                {provisioning ? 'Connecting...' : `Connect ${selectedStores.size} Store(s)`}
              </button>
            </div>
          )}

          {ueStep === 'done' && (
            <p className="text-sm text-green-700 font-medium">Uber Eats connected successfully!</p>
          )}
        </div>
      )}

      {/* Demo info */}
      <div className="mt-8 bg-gray-50 border border-gray-200 rounded-2xl p-4">
        <h3 className="text-xs font-semibold text-gray-500 uppercase mb-2">Demo Information</h3>
        <div className="space-y-1 text-xs text-gray-500">
          <p>API: <span className="font-mono text-gray-700">http://localhost:8080</span></p>
          <p>Uber Eats Sandbox: <span className="font-mono text-gray-700">test-api.uber.com</span></p>
          <p>All actions are executed against the Uber Eats sandbox</p>
        </div>
      </div>
    </div>
  )
}
