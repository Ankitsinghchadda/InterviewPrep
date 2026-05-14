import { api } from './api'
import type { ApiEnvelope, Difficulty, Question } from '@/types'

export interface ListQuestionsParams {
  categories?: string[] // slugs
  difficulty?: Difficulty
  mine?: boolean
  limit?: number
}

export async function listQuestions(params: ListQuestionsParams = {}): Promise<Question[]> {
  const search: Record<string, string> = {}
  if (params.categories?.length) search.categories = params.categories.join(',')
  if (params.difficulty) search.difficulty = params.difficulty
  if (params.mine) search.mine = 'true'
  if (params.limit) search.limit = String(params.limit)
  const { data } = await api.get<ApiEnvelope<Question[]>>('/questions', { params: search })
  return data.data ?? []
}

export async function getQuestion(id: string): Promise<Question | null> {
  const { data } = await api.get<ApiEnvelope<Question>>(`/questions/${id}`)
  return data.data ?? null
}

export interface CreateQuestionInput {
  title: string
  body?: string
  answer: string
  difficulty?: Difficulty
  categories?: string[] // slugs
}

export async function createQuestion(input: CreateQuestionInput): Promise<Question | null> {
  const { data } = await api.post<ApiEnvelope<Question>>('/questions', input)
  return data.data ?? null
}

export async function deleteQuestion(id: string): Promise<void> {
  await api.delete(`/questions/${id}`)
}

export async function listRecommendedQuestions(): Promise<Question[]> {
  const { data } = await api.get<ApiEnvelope<Question[]>>('/questions/recommended')
  return data.data ?? []
}

export async function generateQuestionAudio(id: string): Promise<string> {
  const { data } = await api.post<ApiEnvelope<{ audioUrl: string }>>(
    `/questions/${id}/audio`,
  )
  if (!data.data?.audioUrl) throw new Error(data.error || 'failed to generate audio')
  return data.data.audioUrl
}
