import { api } from './api'
import type { ApiEnvelope, Category, CategoryKind } from '@/types'

export async function listCategories(kind?: CategoryKind): Promise<Category[]> {
  const { data } = await api.get<ApiEnvelope<Category[]>>('/categories', {
    params: kind ? { kind } : undefined,
  })
  return data.data ?? []
}

export async function getCategory(slug: string): Promise<Category | null> {
  const { data } = await api.get<ApiEnvelope<Category>>(`/categories/${slug}`)
  return data.data ?? null
}
