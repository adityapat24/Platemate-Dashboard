import client from './client'
import type { Action, PromoParams } from './types'

interface ExecuteActionParams {
  action_type: string
  dish_mapping_id?: string
  params?: Record<string, unknown>
}

interface ActionResult {
  action_id: string
  status: string
  result: {
    platform: string
    status: string
    message: string
    metadata?: Record<string, unknown>
  }
}

export async function executeAction(payload: ExecuteActionParams): Promise<ActionResult> {
  const { data } = await client.post<ActionResult>('/actions/execute', payload)
  return data
}

export async function removeDish(dishMappingId: string): Promise<ActionResult> {
  return executeAction({ action_type: 'REMOVE_DISH', dish_mapping_id: dishMappingId })
}

export async function restoreDish(dishMappingId: string): Promise<ActionResult> {
  return executeAction({ action_type: 'RESTORE_DISH', dish_mapping_id: dishMappingId })
}

export async function changePrice(dishMappingId: string, newPriceCents: number): Promise<ActionResult> {
  return executeAction({
    action_type: 'CHANGE_PRICE',
    dish_mapping_id: dishMappingId,
    params: { new_price_cents: newPriceCents },
  })
}

export async function runPromo(dishMappingId: string, promoParams: PromoParams): Promise<ActionResult> {
  return executeAction({
    action_type: 'RUN_PROMO',
    dish_mapping_id: dishMappingId,
    params: promoParams as unknown as Record<string, unknown>,
  })
}

export async function getAction(id: string): Promise<Action> {
  const { data } = await client.get<Action>(`/actions/${id}`)
  return data
}

export async function listActions(filters?: { action_type?: string; status?: string }): Promise<Action[]> {
  const { data } = await client.get<{ actions: Action[] }>('/actions', { params: filters })
  return data.actions
}

export async function rollbackAction(id: string): Promise<{ status: string; message: string }> {
  const { data } = await client.post(`/actions/${id}/rollback`)
  return data
}
