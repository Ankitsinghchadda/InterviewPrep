import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createCategory,
  getPublicCategory,
  listCategories,
  listPublicCategories,
  type CreateCategoryInput,
  type PublicCategoryDetail,
} from '@/services/categories'
import {
  createQuestion,
  deleteQuestion,
  findSimilarQuestions,
  generateAnswerDraft,
  generateQuestionExplanation,
  generateQuestions,
  getPublicQuestion,
  getQuestion,
  getSmartRecommendations,
  listQuestions,
  type CreateQuestionInput,
  type FindSimilarInput,
  type GenerateAnswerInput,
  type GenerateQuestionsInput,
  type ListQuestionsParams,
  type QuestionExplanation,
} from '@/services/questions'
import { getStatsOverview } from '@/services/stats'
import {
  getProfile,
  upsertProfile,
  uploadResume,
  type UpsertProfileInput,
} from '@/services/profile'
import { listMyInterviews } from '@/services/interviews'
import { getSubmission, streamSubmission, submitAnswer } from '@/services/submissions'
import {
  completeInterview,
  getInterview,
  nextLiveQuestion,
  startInterview,
  submitInterviewAnswer,
  type StartInterviewInput,
} from '@/services/interviews'
import type {
  Category,
  CategoryKind,
  Interview,
  Profile,
  Question,
  SimilarQuestion,
  SmartRecommendations,
  StatsOverview,
  Submission,
} from '@/types'

export interface RecordingPayload {
  blob: Blob
  transcript?: string
}

export function useCategories(kind?: CategoryKind) {
  return useQuery({
    queryKey: ['categories', kind ?? 'all'],
    queryFn: () => listCategories(kind),
    staleTime: 5 * 60_000,
  })
}

// usePublicCategories: no-auth list of all categories. Powers the public
// /topics index page.
export function usePublicCategories() {
  return useQuery<Category[]>({
    queryKey: ['categories', 'public'],
    queryFn: () => listPublicCategories(),
    staleTime: 5 * 60_000,
  })
}

// usePublicCategory: no-auth fetch of a single category + its public
// questions. Powers /topics/:slug (the topic landing page).
export function usePublicCategory(slug: string | undefined) {
  return useQuery<PublicCategoryDetail | null>({
    queryKey: ['categories', 'public', slug],
    queryFn: () => getPublicCategory(slug!),
    enabled: Boolean(slug),
    staleTime: 5 * 60_000,
  })
}

// useCreateCategory is admin-only on the server. The button that calls it is
// only rendered for admins, but a 403 here is still possible and should be
// surfaced to the caller via the standard mutation error path.
export function useCreateCategory() {
  const qc = useQueryClient()
  return useMutation<Category, Error, CreateCategoryInput>({
    mutationFn: (input) => createCategory(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['categories'] })
    },
  })
}

export function useQuestions(params: ListQuestionsParams) {
  return useQuery({
    queryKey: ['questions', params],
    queryFn: () => listQuestions(params),
  })
}

export function useQuestion(id: string | undefined) {
  return useQuery({
    queryKey: ['question', id],
    queryFn: () => getQuestion(id!),
    enabled: Boolean(id),
  })
}

// usePublicQuestion fetches a question via the no-auth public endpoint.
// Accepts either a UUID or a slug. Used on /questions/:id which is rendered
// for both signed-in and signed-out visitors (the page itself gates the
// answer/practice surfaces based on auth status).
export function usePublicQuestion(idOrSlug: string | undefined) {
  return useQuery({
    queryKey: ['question', 'public', idOrSlug],
    queryFn: () => getPublicQuestion(idOrSlug!),
    enabled: Boolean(idOrSlug),
    staleTime: 5 * 60_000,
  })
}

export function useCreateQuestion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateQuestionInput) => createQuestion(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['questions'] })
    },
  })
}

// useSimilarQuestions powers the live "looks similar to" panel in the create
// dialog. Debounces the title+body so we don't hammer the embeddings API on
// every keystroke, and short-circuits when the title is shorter than 8 chars.
export function useSimilarQuestions(input: FindSimilarInput, debounceMs = 400) {
  const [debounced, setDebounced] = useState<FindSimilarInput>(input)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(input), debounceMs)
    return () => clearTimeout(t)
  }, [input.title, input.body, debounceMs])

  const title = debounced.title.trim()
  return useQuery<SimilarQuestion[]>({
    queryKey: ['questions', 'similar', title, (debounced.body ?? '').trim()],
    queryFn: () => findSimilarQuestions(debounced),
    enabled: title.length >= 8,
    staleTime: 30_000,
  })
}

// useGenerateAnswerDraft drafts a reference answer for a question the user is
// composing. Used by the "Generate" button; the caller writes the result back
// into the form's textarea.
export function useGenerateAnswerDraft() {
  return useMutation<string, Error, GenerateAnswerInput>({
    mutationFn: (input) => generateAnswerDraft(input),
  })
}

// useGenerateQuestions asks the server to AI-author a batch of questions for
// the given categories and persist them as public catalog rows. The empty
// state on the Questions page uses this so users can fill skills with no
// curated content. Invalidates the questions list on success.
export function useGenerateQuestions() {
  const qc = useQueryClient()
  return useMutation<Question[], Error, GenerateQuestionsInput>({
    mutationFn: (input) => generateQuestions(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['questions'] })
    },
  })
}

// useGenerateExplanation: lazily generates the learner-facing explanation
// (summary + markdown body, possibly with a mermaid diagram) and folds the
// result into the cached question so the UI re-renders without a refetch.
export function useGenerateExplanation(questionId: string) {
  const qc = useQueryClient()
  return useMutation<QuestionExplanation, Error>({
    mutationFn: () => generateQuestionExplanation(questionId),
    onSuccess: (data) => {
      qc.setQueryData<Question | null>(['question', questionId], (prev) =>
        prev
          ? {
              ...prev,
              explanationSummary: data.summary,
              explanationMarkdown: data.markdown,
            }
          : prev,
      )
    },
  })
}

export function useDeleteQuestion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteQuestion(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['questions'] })
    },
  })
}

// useSubmitAnswer: standalone Practice mode upload.
export function useSubmitAnswer(questionId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ blob, transcript }: RecordingPayload) =>
      submitAnswer(questionId, blob, transcript),
    onSuccess: (sub) => {
      qc.setQueryData<Submission>(['submission', sub.id], sub)
    },
  })
}

// usePollSubmission: polls every 1.5s until the agent finishes (or fails).
// Kept as a fallback for callers that don't want streaming.
export function usePollSubmission(id: string | null | undefined) {
  return useQuery<Submission | null>({
    queryKey: ['submission', id],
    queryFn: () => getSubmission(id!),
    enabled: Boolean(id),
    refetchInterval: (q) => {
      const s = q.state.data as Submission | null | undefined
      if (!s) return 1500
      if (s.status === 'complete' || s.status === 'failed') return false
      return 1500
    },
    staleTime: 0,
  })
}

// useStreamSubmission: subscribes via SSE for live transcript + review tokens.
// Returns a stable shape the FeedbackCard can consume:
//   - submission: current snapshot from the DB (after review_done it's the final)
//   - transcript: server-confirmed transcript (after STT or client-supplied path)
//   - reviewText: running concatenation of review_token chunks while streaming
//   - status: derived ("connecting" | "transcribing" | "reviewing" | "complete" | "failed")
//   - errorMessage: terminal error from the stream, if any
export interface StreamSubmissionState {
  submission: Submission | null
  transcript: string
  reviewText: string
  status: 'connecting' | 'transcribing' | 'reviewing' | 'complete' | 'failed'
  errorMessage: string | null
}

const INITIAL_STREAM_STATE: StreamSubmissionState = {
  submission: null,
  transcript: '',
  reviewText: '',
  status: 'connecting',
  errorMessage: null,
}

export function useStreamSubmission(id: string | null | undefined): StreamSubmissionState {
  const [state, setState] = useState<StreamSubmissionState>(INITIAL_STREAM_STATE)
  // React's documented pattern for "reset state when a prop changes": track
  // the prior id in state and reset alongside it during render. This avoids
  // both the setState-in-effect anti-pattern and refs-during-render.
  const [lastId, setLastId] = useState<string | null | undefined>(id)
  if (lastId !== id) {
    setLastId(id)
    setState(INITIAL_STREAM_STATE)
  }

  useEffect(() => {
    if (!id) return

    // Seed with an initial snapshot so we have a row to show even if the SSE
    // events take a beat to start (the goroutine may already have completed
    // when we open the stream, in which case the handler emits a one-shot
    // review_done immediately).
    let cancelled = false
    void getSubmission(id).then((s) => {
      if (cancelled || !s) return
      setState((prev) => ({
        ...prev,
        submission: s,
        transcript: s.transcript ?? prev.transcript,
        status:
          s.status === 'complete'
            ? 'complete'
            : s.status === 'failed'
              ? 'failed'
              : prev.status,
        errorMessage: s.errorMessage ?? prev.errorMessage,
      }))
    })

    const unsubscribe = streamSubmission(id, {
      onTranscript: (text) => {
        setState((prev) => ({ ...prev, transcript: text, status: 'reviewing' }))
      },
      onReviewToken: (delta) => {
        setState((prev) => ({
          ...prev,
          reviewText: prev.reviewText + delta,
          status: 'reviewing',
        }))
      },
      onReviewDone: (final) => {
        setState((prev) => ({
          ...prev,
          submission: final,
          transcript: final.transcript ?? prev.transcript,
          status: final.status === 'failed' ? 'failed' : 'complete',
          errorMessage: final.errorMessage ?? null,
        }))
      },
      onError: (err) => {
        setState((prev) => ({
          ...prev,
          status: 'failed',
          errorMessage: err,
        }))
      },
    })

    return () => {
      cancelled = true
      unsubscribe()
    }
  }, [id])

  return state
}

// ---- Interview hooks ------------------------------------------------------

export function useStartInterview() {
  return useMutation({
    mutationFn: (input: StartInterviewInput) => startInterview(input),
  })
}

export function useInterview(id: string | undefined) {
  return useQuery<Interview | null>({
    queryKey: ['interview', id],
    queryFn: () => getInterview(id!),
    enabled: Boolean(id),
    staleTime: 0,
  })
}

export function useSubmitInterviewAnswer(interviewId: string, questionId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ blob, transcript }: RecordingPayload) =>
      submitInterviewAnswer(interviewId, questionId, blob, transcript),
    onSuccess: (sub) => {
      qc.setQueryData<Submission>(['submission', sub.id], sub)
      // Nudge the interview cache so the question count updates.
      void qc.invalidateQueries({ queryKey: ['interview', interviewId] })
    },
  })
}

export function useSmartRecommendations() {
  return useQuery<SmartRecommendations>({
    queryKey: ['questions', 'recommendations'],
    queryFn: () => getSmartRecommendations(),
    staleTime: 60_000,
  })
}

export function useStatsOverview() {
  return useQuery<StatsOverview | null>({
    queryKey: ['stats', 'overview'],
    queryFn: () => getStatsOverview(),
    staleTime: 60_000,
  })
}

export function useMyInterviews() {
  return useQuery<Interview[]>({
    queryKey: ['interviews', 'mine'],
    queryFn: () => listMyInterviews(),
    staleTime: 30_000,
  })
}

// ---- Profile hooks --------------------------------------------------------

export function useProfile() {
  return useQuery<Profile | null>({
    queryKey: ['profile'],
    queryFn: () => getProfile(),
    staleTime: 60_000,
  })
}

export function useUpsertProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: UpsertProfileInput) => upsertProfile(input),
    onSuccess: (p) => {
      qc.setQueryData<Profile>(['profile'], p)
      void qc.invalidateQueries({ queryKey: ['questions', 'recommendations'] })
      void qc.invalidateQueries({ queryKey: ['stats', 'overview'] })
    },
  })
}

export function useUploadResume() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => uploadResume(file),
    onSuccess: (p) => {
      qc.setQueryData<Profile>(['profile'], p)
    },
  })
}

export function useCompleteInterview(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => completeInterview(id),
    onSuccess: (iv) => {
      qc.setQueryData<Interview>(['interview', id], iv)
    },
  })
}

// useNextLiveQuestion: live-mode-only — advances the interview by one turn.
// Server generates the next question from the running transcript + the
// caller's profile/resume. Returns wrap=true when the agent has decided the
// interview should end.
export function useNextLiveQuestion(interviewId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => nextLiveQuestion(interviewId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['interview', interviewId] })
    },
  })
}
