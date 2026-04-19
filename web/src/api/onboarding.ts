import client from './client'
import type { PlatformConnection, UberEatsStore } from './types'

export async function getUberAuthURL(): Promise<{ auth_url: string; state: string }> {
  const { data } = await client.get('/onboarding/ubereats/auth-url')
  return data
}

export async function getUberStores(): Promise<UberEatsStore[]> {
  const { data } = await client.get<{ stores: UberEatsStore[] }>('/onboarding/ubereats/stores')
  return data.stores
}

export async function provisionUberStores(storeIds: string[]): Promise<{ provisioned: string[]; failed: string[] }> {
  const { data } = await client.post('/onboarding/ubereats/provision', {
    store_ids: storeIds,
    webhook_url: `${window.location.origin}/api/webhooks/ubereats`,
  })
  return data
}

export async function getUberConnectionStatus(): Promise<PlatformConnection> {
  const { data } = await client.get<PlatformConnection>('/onboarding/ubereats/status')
  return data
}

export async function getAllConnectionStatuses(): Promise<PlatformConnection[]> {
  const { data } = await client.get<{ connections: PlatformConnection[] }>('/onboarding/status')
  return data.connections
}
