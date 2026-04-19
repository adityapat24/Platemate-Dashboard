import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import client from '../api/client'

export default function UberCallback() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>('processing')
  const [message, setMessage] = useState('')

  useEffect(() => {
    const code = searchParams.get('code')
    const state = searchParams.get('state')
    const error = searchParams.get('error')

    if (error) {
      setStatus('error')
      setMessage(`Authorization denied: ${error}`)
      return
    }

    if (!code || !state) {
      setStatus('error')
      setMessage('Missing code or state in callback')
      return
    }

    // Send code + state to backend.
    client.get('/onboarding/ubereats/callback', { params: { code, state } })
      .then(() => {
        setStatus('success')
        setMessage('Uber Eats authorized successfully!')
        setTimeout(() => navigate('/settings'), 2000)
      })
      .catch((e) => {
        const msg = e?.response?.data?.error || 'Authorization failed'
        setStatus('error')
        setMessage(msg)
      })
  }, [searchParams, navigate])

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center">
      <div className="bg-white rounded-2xl shadow-lg p-8 w-full max-w-sm text-center">
        {status === 'processing' && (
          <>
            <div className="text-4xl mb-4 animate-pulse">🔄</div>
            <h1 className="text-base font-semibold text-gray-900">Connecting Uber Eats...</h1>
            <p className="text-sm text-gray-500 mt-2">Please wait while we complete authorization</p>
          </>
        )}
        {status === 'success' && (
          <>
            <div className="text-4xl mb-4">✅</div>
            <h1 className="text-base font-semibold text-gray-900">Connected!</h1>
            <p className="text-sm text-gray-500 mt-2">{message}</p>
            <p className="text-xs text-gray-400 mt-2">Redirecting to settings...</p>
          </>
        )}
        {status === 'error' && (
          <>
            <div className="text-4xl mb-4">❌</div>
            <h1 className="text-base font-semibold text-gray-900">Authorization Failed</h1>
            <p className="text-sm text-red-600 mt-2">{message}</p>
            <button
              onClick={() => navigate('/settings')}
              className="mt-4 px-5 py-2 bg-orange-500 hover:bg-orange-600 text-white text-sm font-semibold rounded-lg"
            >
              Back to Settings
            </button>
          </>
        )}
      </div>
    </div>
  )
}
