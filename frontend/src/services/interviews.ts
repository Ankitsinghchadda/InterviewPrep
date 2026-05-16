import { api } from './api'
import type { ApiEnvelope, Interview, Question, Submission } from '@/types'

export interface StartInterviewInput {
  mode?: 'topic' | 'adaptive' | 'live'
  categories: string[] // slugs (ignored in adaptive/live mode)
  count?: number // topic/adaptive only
  durationMinutes?: number // live mode: 15|30|45
  jobDescription?: string // live mode only, optional
}

export interface NextLiveQuestionResponse {
  question?: Question
  wrap: boolean
  timeRemainingSec: number
}

export async function startInterview(input: StartInterviewInput): Promise<Interview> {
  const { data } = await api.post<ApiEnvelope<Interview>>('/interviews', input)
  if (!data.data) throw new Error(data.error || 'failed to start interview')
  return data.data
}

export async function getInterview(id: string): Promise<Interview | null> {
  const { data } = await api.get<ApiEnvelope<Interview>>(`/interviews/${id}`)
  return data.data ?? null
}

export async function listMyInterviews(): Promise<Interview[]> {
  const { data } = await api.get<ApiEnvelope<Interview[]>>('/interviews')
  return data.data ?? []
}

export async function submitInterviewAnswer(
  interviewId: string,
  questionId: string,
  audio: Blob,
  transcript?: string,
): Promise<Submission> {
  const form = new FormData()
  const ext = mimeToExt(audio.type)
  form.append('audio', audio, `answer.${ext}`)
  if (transcript) form.append('transcript', transcript)
  const { data } = await api.post<ApiEnvelope<Submission>>(
    `/interviews/${interviewId}/questions/${questionId}/answer`,
    form,
    { headers: { 'Content-Type': 'multipart/form-data' } },
  )
  if (!data.data) throw new Error(data.error || 'submission failed')
  return data.data
}

export async function completeInterview(id: string): Promise<Interview> {
  const { data } = await api.post<ApiEnvelope<Interview>>(`/interviews/${id}/complete`)
  if (!data.data) throw new Error(data.error || 'failed to complete interview')
  return data.data
}

export async function nextLiveQuestion(id: string): Promise<NextLiveQuestionResponse> {
  const { data } = await api.post<ApiEnvelope<NextLiveQuestionResponse>>(
    `/interviews/${id}/next-question`,
  )
  if (!data.data) throw new Error(data.error || 'failed to get next question')
  return data.data
}

function mimeToExt(mime: string): string {
  if (!mime) return 'webm'
  if (mime.includes('webm')) return 'webm'
  if (mime.includes('ogg')) return 'ogg'
  if (mime.includes('mp4') || mime.includes('m4a')) return 'm4a'
  if (mime.includes('wav')) return 'wav'
  if (mime.includes('mpeg')) return 'mp3'
  return 'bin'
}
