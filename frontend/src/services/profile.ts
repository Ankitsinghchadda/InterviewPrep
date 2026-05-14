import { api } from './api'
import type { ApiEnvelope, Profile, Seniority } from '@/types'

export async function getProfile(): Promise<Profile | null> {
  const { data } = await api.get<ApiEnvelope<Profile | null>>('/profile')
  return data.data ?? null
}

export interface UpsertProfileInput {
  targetRole?: string
  yearsExperience?: number
  seniority?: Seniority
  currentRole?: string
  techStack?: string[]
  targetCompanies?: string[]
  goals?: string
  markOnboarded?: boolean
}

export async function upsertProfile(input: UpsertProfileInput): Promise<Profile> {
  const { data } = await api.put<ApiEnvelope<Profile>>('/profile', input)
  if (!data.data) throw new Error(data.error || 'failed to save profile')
  return data.data
}

export async function uploadResume(file: File): Promise<Profile> {
  const form = new FormData()
  form.append('resume', file, file.name)
  const { data } = await api.post<ApiEnvelope<Profile>>('/profile/resume', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 90_000,
  })
  if (!data.data) throw new Error(data.error || 'resume parsing failed')
  return data.data
}
