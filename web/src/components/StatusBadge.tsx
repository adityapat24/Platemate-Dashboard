interface Props {
  status: string
  size?: 'sm' | 'md'
}

const statusConfig: Record<string, { bg: string; text: string; dot: string; label?: string }> = {
  connected: { bg: 'bg-green-50', text: 'text-green-700', dot: 'bg-green-500', label: 'Connected' },
  pending: { bg: 'bg-yellow-50', text: 'text-yellow-700', dot: 'bg-yellow-500', label: 'Pending' },
  error: { bg: 'bg-red-50', text: 'text-red-700', dot: 'bg-red-500', label: 'Error' },
  disconnected: { bg: 'bg-gray-100', text: 'text-gray-500', dot: 'bg-gray-400', label: 'Not Connected' },
  not_connected: { bg: 'bg-gray-100', text: 'text-gray-500', dot: 'bg-gray-400', label: 'Not Connected' },
  complete: { bg: 'bg-green-50', text: 'text-green-700', dot: 'bg-green-500', label: 'Complete' },
  partial: { bg: 'bg-yellow-50', text: 'text-yellow-700', dot: 'bg-yellow-500', label: 'Partial' },
  failed: { bg: 'bg-red-50', text: 'text-red-700', dot: 'bg-red-500', label: 'Failed' },
  success: { bg: 'bg-green-50', text: 'text-green-700', dot: 'bg-green-500', label: 'Success' },
  suspended: { bg: 'bg-orange-50', text: 'text-orange-700', dot: 'bg-orange-500', label: 'Suspended' },
  active: { bg: 'bg-green-50', text: 'text-green-700', dot: 'bg-green-500', label: 'Active' },
}

export default function StatusBadge({ status, size = 'md' }: Props) {
  const cfg = statusConfig[status.toLowerCase()] ?? {
    bg: 'bg-gray-100',
    text: 'text-gray-600',
    dot: 'bg-gray-400',
    label: status,
  }
  const sizeClasses = size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-xs'

  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full font-medium ${cfg.bg} ${cfg.text} ${sizeClasses}`}>
      <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${cfg.dot}`} />
      {cfg.label ?? status}
    </span>
  )
}
