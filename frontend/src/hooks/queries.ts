import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { listCategories } from '@/services/categories'
import {
  createQuestion,
  deleteQuestion,
  getQuestion,
  listQuestions,
  listRecommendedQuestions,
  type CreateQuestionInput,
  type ListQuestionsParams,
} from '@/services/questions'
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
import type { CategoryKind, Interview, Profile, Submission } from '@/types'

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

export function useCreateQuestion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateQuestionInput) => createQuestion(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['questions'] })
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

export function useRecommendedQuestions() {
  return useQuery({
    queryKey: ['questions', 'recommended'],
    queryFn: () => listRecommendedQuestions(),
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
      void qc.invalidateQueries({ queryKey: ['questions', 'recommended'] })
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
