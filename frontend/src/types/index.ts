export type Seniority = '' | 'junior' | 'mid' | 'senior' | 'staff' | 'principal'

export interface Profile {
  userId: string
  targetRole: string
  yearsExperience: number
  seniority: Seniority
  currentRole: string
  techStack: string[]
  targetCompanies: string[]
  goals: string
  resumeText?: string
  resumeFilename?: string
  onboardedAt?: string
  createdAt: string
  updatedAt: string
}

export type InterviewMode = 'topic' | 'adaptive' | 'live'

export type CategoryKind = 'role' | 'topic'

export interface Category {
  id: string
  slug: string
  name: string
  kind: CategoryKind
  description: string
  icon: string
  sortOrder: number
  createdAt: string
}

export type Difficulty = 'easy' | 'medium' | 'hard'

export interface Question {
  id: string
  slug?: string
  title: string
  body: string
  answer: string
  difficulty: Difficulty
  answerAudioUrl?: string
  ownerId?: string
  isPublic: boolean
  source: 'curated' | 'user' | 'adaptive' | 'live'
  intent?: string
  categories: string[] // slugs
  createdAt: string
}

export type SubmissionStatus =
  | 'pending'
  | 'transcribing'
  | 'reviewing'
  | 'complete'
  | 'failed'

export interface Submission {
  id: string
  userId: string
  questionId: string
  interviewId?: string
  audioUrl?: string
  transcript?: string
  feedback?: string
  strengths?: string[]
  improvements?: string[]
  score?: number
  status: SubmissionStatus
  errorMessage?: string
  createdAt: string
  updatedAt: string
}

export type InterviewStatus = 'in_progress' | 'completed' | 'abandoned'

export interface Interview {
  id: string
  userId: string
  mode: InterviewMode
  categoryIds: string[]
  status: InterviewStatus
  score?: number
  summary?: string
  startedAt: string
  finishedAt?: string
  durationSeconds?: number // live mode: 900|1800|2700
  questions?: Question[]
  submissions?: Submission[]
}

export interface ApiEnvelope<T> {
  data?: T
  error?: string
}

export interface User {
  id: string
  email: string
  emailVerified: boolean
  name: string
  pictureUrl?: string
  createdAt: string
  updatedAt: string
  lastLoginAt?: string
}
