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
  /** TTS audio of the interviewer reading the question aloud. Only set on
   *  live-interview questions, and arrives a beat after the question itself
   *  since synthesis happens in the background. */
  promptAudioUrl?: string
  explanationSummary?: string
  explanationMarkdown?: string
  ownerId?: string
  isPublic: boolean
  source: 'curated' | 'user' | 'adaptive' | 'live'
  intent?: string
  categories: string[] // slugs
  createdAt: string
  // Only populated when the row came from `/questions?q=…` (hybrid search).
  score?: number
  snippet?: string // HTML with <mark> tags around matched terms
}

// SimilarQuestion is a Question augmented with its cosine similarity (0..1) to
// the query the user is typing. Returned by the dedup endpoints; the UI shows
// the score as a "92% match" badge.
export interface SimilarQuestion extends Question {
  similarity: number
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

// Collection is a user-owned named list of questions. Every user has a
// default "Saved" collection (isDefault = true) that acts as a one-click
// bookmark; users can create additional named collections.
export interface Collection {
  id: string
  userId: string
  name: string
  description?: string
  color?: string
  isDefault: boolean
  questionCount: number
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

// ---- Dashboard stats ------------------------------------------------------

export interface VolumeStats {
  totalSubmissions: number
  uniqueQuestions: number
  interviewsStarted: number
  interviewsCompleted: number
  submissionsLast30Days: number
}

export interface ScoringStats {
  averageScore: number | null
  averageLast30: number | null
  averagePrior30: number | null
  bestScore: number | null
  scoredCount: number
}

export interface StreakStats {
  current: number
  longest: number
  practicedToday: boolean
}

export interface TrendPoint {
  day: string // YYYY-MM-DD
  submissions: number
  avgScore: number | null
}

export interface CategoryScore {
  slug: string
  name: string
  submissions: number
  avgScore: number
}

export interface CategoryStrengths {
  strong: CategoryScore[]
  weak: CategoryScore[]
}

export interface ThemeCount {
  theme: string
  count: number
}

export interface ThemeStats {
  strengths: ThemeCount[]
  improvements: ThemeCount[]
}

export interface DifficultyBucket {
  difficulty: Difficulty
  submissions: number
  avgScore: number | null
}

export interface GoalAlignment {
  targetRole: string
  onTargetSubmissions: number
  offTargetSubmissions: number
  alignmentPercent: number
}

export interface StatsOverview {
  volume: VolumeStats
  scoring: ScoringStats
  streak: StreakStats
  trend: TrendPoint[]
  categories: CategoryStrengths
  themes: ThemeStats
  difficultyDistribution: DifficultyBucket[]
  goalAlignment: GoalAlignment
  generatedAt: string
}

// ---- Smart recommendations ------------------------------------------------

export interface RecommendationItem {
  question: Question
  reason: string
}

export interface SmartRecommendations {
  weakAreas: RecommendationItem[]
  levelUp: RecommendationItem[]
  goalGaps: RecommendationItem[]
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
  // Derived from the backend's ADMIN_EMAILS allow-list. UI hint only — every
  // admin route is independently enforced server-side.
  isAdmin?: boolean

  plan: 'free' | 'pro'
  planPeriod?: 'monthly' | 'biannual' | ''
  planStartedAt?: string
  planExpiresAt?: string
}

// UsageKind mirrors backend billing.Kind. Keep these strings in sync with
// internal/billing/quota.go.
export type UsageKind =
  | 'recording_review'
  | 'mock_basic'
  | 'mock_live'
  | 'question_add'
  | 'question_gen'
  | 'answer_gen'
  | 'explanation'
  | 'tts'

export interface UsageRow {
  used: number
  limit: number // -1 = unlimited
  remaining: number // -1 = unlimited
  blocked?: boolean // true when kind is not available on the user's plan at all
}

export interface UsageSnapshot {
  plan: 'free' | 'pro'
  planExpiresAt?: string
  windowStart: string
  quotas: Record<UsageKind, UsageRow>
}

// PaywallReason is what the interceptor stashes when a 402/403 lands.
// 'quota_exceeded' fires when a free user hits the weekly cap on a kind
// they're allowed in principle. 'plan_required' fires when the kind is
// blocked entirely (e.g. mock_live for free).
export interface PaywallReason {
  error: 'quota_exceeded' | 'plan_required'
  kind: UsageKind
  requiredPlan?: 'pro'
  plan?: 'free' | 'pro'
}
