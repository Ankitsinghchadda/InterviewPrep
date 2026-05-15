import { api } from './api'
import type { ApiEnvelope, Category, CategoryKind, Question } from '@/types'

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

// listPublicCategories: no-auth list of every category. Used by the public
// /topics route (anonymous visitors + search engines).
export async function listPublicCategories(): Promise<Category[]> {
  const { data } = await api.get<ApiEnvelope<Category[]>>('/public/categories')
  return data.data ?? []
}

export interface PublicCategoryDetail {
  category: Category
  questions: Question[]
}

// getPublicCategory: no-auth fetch of one category + all of its public
// questions. Used by /topics/:slug — the highest-value SEO landing page
// (targets queries like "javascript interview questions").
export async function getPublicCategory(slug: string): Promise<PublicCategoryDetail | null> {
  const { data } = await api.get<ApiEnvelope<PublicCategoryDetail>>(
    `/public/categories/${encodeURIComponent(slug)}`,
  )
  return data.data ?? null
}

export interface CreateCategoryInput {
  slug: string
  name: string
  kind: CategoryKind
  description?: string
  icon?: string
  sortOrder?: number
}

// Admin-only: backend enforces ADMIN_EMAILS gate.
export async function createCategory(input: CreateCategoryInput): Promise<Category> {
  const { data } = await api.post<ApiEnvelope<Category>>('/categories', input)
  if (!data.data) throw new Error(data.error || 'failed to create category')
  return data.data
}
