import client from './client'
import type { Dish } from './types'

export async function listDishes(): Promise<Dish[]> {
  const { data } = await client.get<{ dishes: Dish[] }>('/dishes')
  return data.dishes
}

export async function getDish(id: string): Promise<Dish> {
  const { data } = await client.get<Dish>(`/dishes/${id}`)
  return data
}

export async function syncMenu(): Promise<{ synced: number; message: string }> {
  const { data } = await client.post('/dishes/sync')
  return data
}
