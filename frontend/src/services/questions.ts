import axios from 'axios'

import { api } from './api'
import type {
  ApiEnvelope,
  Difficulty,
  Question,
  SimilarQuestion,
  SmartRecommendations,
} from '@/types'

export interface ListQuestionsParams {
  categories?: string[] // slugs
  difficulty?: Difficulty
  mine?: boolean
  limit?: number
  // Free-text query. When set, the API runs hybrid semantic + keyword search
  // and the response carries `score` and `snippet` per row.
  q?: string
  // Only questions that belong to this collection (caller must own it).
  inCollection?: string
}

export async function listQuestions(params: ListQuestionsParams = {}): Promise<Question[]> {
  const search: Record<string, string> = {}
  if (params.categories?.length) search.categories = params.categories.join(',')
  if (params.difficulty) search.difficulty = params.difficulty
  if (params.mine) search.mine = 'true'
  if (params.limit) search.limit = String(params.limit)
  if (params.q && params.q.trim()) search.q = params.q.trim()
  if (params.inCollection) search.in_collection = params.inCollection
  const { data } = await api.get<ApiEnvelope<Question[]>>('/questions', { params: search })
  return data.data ?? []
}

// listCollectionIdsForQuestion returns the ids of the caller's collections
// that already include the given question. Drives the bookmark indicator and
// "Save to..." menu without pulling the full collection rows.
export async function listCollectionIdsForQuestion(id: string): Promise<string[]> {
  const { data } = await api.get<ApiEnvelope<string[]>>(`/questions/${id}/collections`)
  return data.data ?? []
}

// listQuestionSubmissions returns the caller's submission history for one
// question, newest first. Used by QuestionDetail's history panel and the
// in-flight stream resume logic.
export async function listQuestionSubmissions(id: string): Promise<import('@/types').Submission[]> {
  const { data } = await api.get<ApiEnvelope<import('@/types').Submission[]>>(
    `/questions/${id}/submissions`,
  )
  return data.data ?? []
}

export async function getQuestion(id: string): Promise<Question | null> {
  const { data } = await api.get<ApiEnvelope<Question>>(`/questions/${id}`)
  return data.data ?? null
}

// getPublicQuestion fetches a public question by UUID id OR slug, WITHOUT
// auth. Used on the public-facing question detail page so search engines and
// signed-out visitors can read the question. Backend enforces is_public=true.
export async function getPublicQuestion(idOrSlug: string): Promise<Question | null> {
  const { data } = await api.get<ApiEnvelope<Question>>(
    `/public/questions/${encodeURIComponent(idOrSlug)}`,
  )
  return data.data ?? null
}

export interface CreateQuestionInput {
  title: string
  body?: string
  // Optional: when blank, the server auto-generates a reference answer.
  answer?: string
  difficulty?: Difficulty
  categories?: string[] // slugs
  // When true, bypass the near-duplicate soft-block (used after the user
  // confirms "Create anyway" on a 409 response).
  force?: boolean
}

// SimilarQuestionConflict is thrown by createQuestion when the server returns
// a 409 because the title is too close to an existing question. Surface this
// to the dialog so it can offer "Use existing" / "Create anyway".
export class SimilarQuestionConflict extends Error {
  readonly matches: SimilarQuestion[]
  constructor(matches: SimilarQuestion[]) {
    super('A similar question already exists.')
    this.name = 'SimilarQuestionConflict'
    this.matches = matches
  }
}

export async function createQuestion(input: CreateQuestionInput): Promise<Question | null> {
  const { force, ...body } = input
  try {
    const { data } = await api.post<ApiEnvelope<Question>>('/questions', body, {
      params: force ? { force: 'true' } : undefined,
    })
    return data.data ?? null
  } catch (err) {
    if (axios.isAxiosError(err) && err.response?.status === 409) {
      const payload = err.response.data?.data as
        | { error?: string; matches?: SimilarQuestion[] }
        | undefined
      throw new SimilarQuestionConflict(payload?.matches ?? [])
    }
    throw err
  }
}

export interface FindSimilarInput {
  title: string
  body?: string
}

export async function findSimilarQuestions(
  input: FindSimilarInput,
): Promise<SimilarQuestion[]> {
  const { data } = await api.post<ApiEnvelope<SimilarQuestion[]>>(
    '/questions/similar',
    input,
  )
  return data.data ?? []
}

export interface GenerateAnswerInput {
  title: string
  body?: string
  difficulty?: Difficulty
  categories?: string[]
}

export async function generateAnswerDraft(input: GenerateAnswerInput): Promise<string> {
  const { data } = await api.post<ApiEnvelope<{ answer: string }>>(
    '/questions/generate-answer',
    input,
  )
  if (!data.data?.answer) {
    throw new Error(data.error || 'failed to generate reference answer')
  }
  return data.data.answer
}

export async function deleteQuestion(id: string): Promise<void> {
  await api.delete(`/questions/${id}`)
}

const EMPTY_RECOMMENDATIONS: SmartRecommendations = {
  weakAreas: [],
  levelUp: [],
  goalGaps: [],
}

export async function getSmartRecommendations(): Promise<SmartRecommendations> {
  const { data } = await api.get<ApiEnvelope<SmartRecommendations>>(
    '/questions/recommendations',
  )
  return data.data ?? EMPTY_RECOMMENDATIONS
}

export interface GenerateQuestionsInput {
  categories: string[] // slugs (at least one required)
  difficulty?: Difficulty
  count?: number // 1..10, server clamps; default 5
}

// generateQuestions asks the server to AI-author a batch of questions for the
// given categories and persist them as public catalog rows. Returns the newly
// created questions (already deduped against the existing library).
export async function generateQuestions(
  input: GenerateQuestionsInput,
): Promise<Question[]> {
  const { data } = await api.post<ApiEnvelope<Question[]>>('/questions/generate', input)
  return data.data ?? []
}

export async function generateQuestionAudio(id: string): Promise<string> {
  const { data } = await api.post<ApiEnvelope<{ audioUrl: string }>>(
    `/questions/${id}/audio`,
  )
  if (!data.data?.audioUrl) throw new Error(data.error || 'failed to generate audio')
  return data.data.audioUrl
}

// generateQuestionPromptAudio synthesizes (or returns cached) the interviewer
// voice reading the question aloud. Used by the live interview runner as a
// fallback when the background-generated URL hasn't appeared yet.
export async function generateQuestionPromptAudio(id: string): Promise<string> {
  const { data } = await api.post<ApiEnvelope<{ audioUrl: string }>>(
    `/questions/${id}/prompt-audio`,
  )
  if (!data.data?.audioUrl) throw new Error(data.error || 'failed to generate audio')
  return data.data.audioUrl
}

export interface QuestionExplanation {
  summary: string
  markdown: string
}

export async function generateQuestionExplanation(id: string): Promise<QuestionExplanation> {
  const { data } = await api.post<ApiEnvelope<QuestionExplanation>>(
    `/questions/${id}/explanation`,
  )
  if (!data.data?.markdown) throw new Error(data.error || 'failed to generate explanation')
  return data.data
}
