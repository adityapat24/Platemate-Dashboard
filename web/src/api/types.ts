// ── Dish types ────────────────────────────────────────────────────────────────

export interface UberEatsPlatformData {
  item_id: string
  store_id: string
  price_cents: number
  active: boolean
  suspended: boolean
  suspend_until?: number
  last_synced: string
}

export interface DishPlatformData {
  uber_eats?: UberEatsPlatformData
  toast?: Record<string, unknown>
  r365?: Record<string, unknown>
  google?: Record<string, unknown>
}

export interface DishAnalytics {
  platemate_review_count: number
  avg_overall: number
  avg_taste: number
  avg_portion: number
  avg_value: number
  reorder_rate: number
  trend_direction: 'improving' | 'declining' | 'stable' | ''
  health_score: number
  last_calculated: string
}

export interface DishFinancials {
  food_cost: number
  margin_percent: number
  weekly_units: number
  weekly_revenue: number
  last_calculated: string
}

export interface Dish {
  id: string
  restaurant_id: string
  canonical_name: string
  category: string
  platforms: DishPlatformData
  analytics: DishAnalytics
  financials: DishFinancials
  created_at: string
  updated_at: string
}

// ── Action types ──────────────────────────────────────────────────────────────

export interface PlatformActionResult {
  platform: string
  status: 'pending' | 'success' | 'failed' | 'manual'
  message?: string
  completed_at?: string
  metadata?: Record<string, unknown>
}

export interface Action {
  id: string
  restaurant_id: string
  action_type: string
  dish_mapping_id?: string
  dish_name: string
  params: Record<string, unknown>
  initiator: string
  status: 'pending' | 'complete' | 'partial' | 'failed'
  results: PlatformActionResult[]
  can_rollback: boolean
  rolled_back: boolean
  created_at: string
  updated_at: string
}

// ── Connection types ──────────────────────────────────────────────────────────

export interface PlatformConnection {
  id?: string
  restaurant_id: string
  platform: string
  status: 'connected' | 'pending' | 'error' | 'disconnected' | 'not_connected'
  store_id?: string
  store_name?: string
  last_sync?: string
  error?: string
}

// ── Uber Eats store ───────────────────────────────────────────────────────────

export interface UberEatsStore {
  id: string
  name: string
  timezone?: string
  location?: {
    address: string
    city: string
    postal_code: string
    country: string
  }
}

// ── Promo config ──────────────────────────────────────────────────────────────

export interface PromoParams {
  promo_type: string
  discount_percent?: number
  discount_amount_cents?: number
  user_group?: string
  unlimited_budget?: boolean
  budget_cents?: number
  external_promo_id?: string
}
