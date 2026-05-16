import { api } from './api'
import type { ApiEnvelope, Collection } from '@/types'

export async function listCollections(): Promise<Collection[]> {
  const { data } = await api.get<ApiEnvelope<Collection[]>>('/collections')
  return data.data ?? []
}

export async function getCollection(id: string): Promise<Collection | null> {
  const { data } = await api.get<ApiEnvelope<Collection>>(`/collections/${id}`)
  return data.data ?? null
}

export interface CreateCollectionInput {
  name: string
  description?: string
  color?: string
}

export async function createCollection(input: CreateCollectionInput): Promise<Collection> {
  const { data } = await api.post<ApiEnvelope<Collection>>('/collections', input)
  if (!data.data) throw new Error(data.error || 'failed to create collection')
  return data.data
}

export interface UpdateCollectionInput {
  name?: string
  description?: string
  color?: string
}

export async function updateCollection(
  id: string,
  input: UpdateCollectionInput,
): Promise<Collection> {
  const { data } = await api.patch<ApiEnvelope<Collection>>(`/collections/${id}`, input)
  if (!data.data) throw new Error(data.error || 'failed to update collection')
  return data.data
}

export async function deleteCollection(id: string): Promise<void> {
  await api.delete(`/collections/${id}`)
}

export async function addQuestionToCollection(
  collectionId: string,
  questionId: string,
): Promise<void> {
  await api.post(`/collections/${collectionId}/questions`, { questionId })
}

export async function removeQuestionFromCollection(
  collectionId: string,
  questionId: string,
): Promise<void> {
  await api.delete(`/collections/${collectionId}/questions/${questionId}`)
}
