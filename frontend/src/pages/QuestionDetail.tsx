import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { ArrowLeft, BookOpen, Loader2, Lock, Mic } from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import {
  useQuestionSubmissions,
  usePublicQuestion,
  useStreamSubmission,
  useSubmitAnswer,
} from '@/hooks/queries'
import { useSEO } from '@/hooks/useSEO'
import type { Difficulty, Submission } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { AnswerExplanation } from '@/components/AnswerExplanation'
import { Recorder, type RecordingPayload } from '@/components/Recorder'
import { FeedbackCard } from '@/components/FeedbackCard'
import { SaveToCollectionMenu } from '@/components/SaveToCollectionMenu'
import { SubmissionHistoryList } from '@/components/SubmissionHistoryList'

const DIFFICULTY_VARIANT: Record<Difficulty, 'success' | 'brand' | 'destructive'> = {
  easy: 'success',
  medium: 'brand',
  hard: 'destructive',
}

export function QuestionDetail() {
  const { id } = useParams<{ id: string }>()
  const location = useLocation()
  const { status: authStatus } = useAuth()
  const isAuthed = authStatus === 'authenticated'
  const { data: question, isLoading, error } = usePublicQuestion(id)

  // Active submission id is now derived state: prefer (in order)
  //   1. an id the user just kicked off in this tab (local state),
  //   2. the most recent in-flight submission on this question from the DB,
  //   3. the localStorage hint (covers the gap between submit success and the
  //      next /submissions list refetch — and re-hydrates after a hard refresh).
  // This means navigating away during streaming and coming back picks up
  // exactly where we left off; the stream hook seeds from /submissions/{id}
  // before the SSE events flow, so the UI repaints instantly.
  const questionKey = question?.id || id || ''
  const localKey = questionKey ? `activeSubmission:${questionKey}` : ''
  const [localSubmissionId, setLocalSubmissionId] = useState<string | null>(() => {
    if (typeof window === 'undefined' || !localKey) return null
    return window.localStorage.getItem(localKey)
  })
  const { data: submissionHistory = [] } = useQuestionSubmissions(
    isAuthed ? questionKey || undefined : undefined,
  )

  const inFlightFromServer = useMemo(
    () => findInFlight(submissionHistory),
    [submissionHistory],
  )
  const submissionId = localSubmissionId || inFlightFromServer?.id || null

  const submit = useSubmitAnswer(id || '')
  const stream = useStreamSubmission(submissionId)

  // Persist + clear the localStorage hint based on the live stream state.
  // We don't drop the hint until the stream reports terminal so back-nav
  // mid-stream stays seamless even before the /submissions list refetches.
  useEffect(() => {
    if (typeof window === 'undefined' || !localKey) return
    if (!submissionId) return
    if (stream.status === 'complete' || stream.status === 'failed') {
      window.localStorage.removeItem(localKey)
      return
    }
    window.localStorage.setItem(localKey, submissionId)
  }, [submissionId, stream.status, localKey])

  const loginHref = `/login?redirect=${encodeURIComponent(location.pathname + location.search)}`

  // Per-page SEO. Canonicalize to slug when present (better than UUID for
  // search engines); fall back to id otherwise. JSON-LD uses Google's QAPage
  // schema — eligible for rich Q&A results in SERPs.
  const slugOrId = question?.slug || question?.id || id || ''
  const description = question
    ? (question.body || `${question.title} — interview question with reference answer and AI feedback.`).slice(0, 158)
    : 'Interview question — 10xInterview'

  // Build a BreadcrumbList alongside QAPage so we're eligible for both
  // breadcrumb rich results AND Q&A rich results. Crawl path:
  // Home → Topic (first category) → Question.
  const primaryCategory = question?.categories?.[0]
  useSEO({
    title: question?.title || 'Question',
    description,
    path: `/questions/${slugOrId}`,
    type: 'article',
    jsonLd: question
      ? [
          {
            '@context': 'https://schema.org',
            '@type': 'QAPage',
            mainEntity: {
              '@type': 'Question',
              name: question.title,
              text: question.body || question.title,
              answerCount: question.answer ? 1 : 0,
              ...(question.answer && {
                acceptedAnswer: {
                  '@type': 'Answer',
                  text: question.answer,
                },
              }),
            },
          },
          {
            '@context': 'https://schema.org',
            '@type': 'BreadcrumbList',
            itemListElement: [
              {
                '@type': 'ListItem',
                position: 1,
                name: 'Home',
                item: 'https://10xinterview.com/',
              },
              ...(primaryCategory
                ? [
                    {
                      '@type': 'ListItem',
                      position: 2,
                      name: primaryCategory,
                      item: `https://10xinterview.com/topics/${primaryCategory}`,
                    },
                    {
                      '@type': 'ListItem',
                      position: 3,
                      name: question.title,
                      item: `https://10xinterview.com/questions/${slugOrId}`,
                    },
                  ]
                : [
                    {
                      '@type': 'ListItem',
                      position: 2,
                      name: question.title,
                      item: `https://10xinterview.com/questions/${slugOrId}`,
                    },
                  ]),
            ],
          },
        ]
      : undefined,
  })

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading question…
      </div>
    )
  }

  if (error || !question) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Question not found</CardTitle>
          <CardDescription>It may have been removed or is not yet public.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant="outline">
            <Link to={isAuthed ? '/questions' : '/'}>
              <ArrowLeft className="size-4" /> Back
            </Link>
          </Button>
        </CardContent>
      </Card>
    )
  }

  const onSubmit = async (payload: RecordingPayload) => {
    try {
      const sub = await submit.mutateAsync(payload)
      setLocalSubmissionId(sub.id)
      if (typeof window !== 'undefined' && localKey) {
        window.localStorage.setItem(localKey, sub.id)
      }
    } catch (err) {
      console.error('submit failed', err)
    }
  }

  const startOver = () => {
    setLocalSubmissionId(null)
    if (typeof window !== 'undefined' && localKey) {
      window.localStorage.removeItem(localKey)
    }
    submit.reset()
  }

  return (
    <article className="space-y-6">
      <div>
        <Link
          to={isAuthed ? '/questions' : '/'}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-4" /> {isAuthed ? 'All questions' : 'Home'}
        </Link>
      </div>

      <header className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={DIFFICULTY_VARIANT[question.difficulty] || 'outline'}>
            {question.difficulty}
          </Badge>
          {question.categories.slice(0, 6).map((slug) => (
            <Badge key={slug} variant="outline" className="text-muted-foreground">
              {slug}
            </Badge>
          ))}
        </div>
        <div className="flex items-start justify-between gap-3">
          <h1 className="min-w-0 break-words text-xl font-bold tracking-tight sm:text-2xl md:text-3xl">{question.title}</h1>
          {isAuthed && questionKey && <SaveToCollectionMenu questionId={questionKey} />}
        </div>
        {question.body && <p className="text-sm text-muted-foreground sm:text-base">{question.body}</p>}
      </header>

      <Tabs defaultValue="answer">
        <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:overflow-visible sm:px-0">
          <TabsList className="w-max min-w-full justify-start sm:w-auto sm:min-w-0">
            <TabsTrigger value="answer" className="whitespace-nowrap">
              <BookOpen className="size-4" /> Reference answer
            </TabsTrigger>
            <TabsTrigger value="practice" className="whitespace-nowrap">
              <Mic className="size-4" /> Practice
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="answer">
          {isAuthed ? (
            <AnswerExplanation question={question} />
          ) : (
            <Card className="border-brand-500/30 bg-brand-500/5">
              <CardHeader>
                <div className="mb-2 inline-flex size-10 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                  <Lock className="size-5" />
                </div>
                <CardTitle>Sign in to view the reference answer</CardTitle>
                <CardDescription>
                  The full answer, AI-generated explanation, and audio walkthrough are
                  unlocked when you sign in. Free — takes 5 seconds with Google.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button asChild>
                  <Link to={loginHref}>Sign in with Google</Link>
                </Button>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="practice" className="space-y-5">
          {!isAuthed ? (
            <Card className="border-brand-500/30 bg-brand-500/5">
              <CardHeader>
                <div className="mb-2 inline-flex size-10 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                  <Lock className="size-5" />
                </div>
                <CardTitle>Sign in to practice with AI feedback</CardTitle>
                <CardDescription>
                  Record your spoken answer (up to 90 seconds) and our AI interviewer scores
                  it with strengths and improvements. Requires a free account.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button asChild>
                  <Link to={loginHref}>Sign in to start practicing</Link>
                </Button>
              </CardContent>
            </Card>
          ) : (
            <>
              {!submissionId && !submit.isPending && (
                <Card>
                  <CardHeader>
                    <div className="mb-2 inline-flex size-10 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                      <Mic className="size-5" />
                    </div>
                    <CardTitle>Record your answer</CardTitle>
                    <CardDescription>
                      Speak your answer aloud (up to 90 seconds). When you submit, we’ll transcribe it and
                      the AI interviewer will score it with strengths and improvements.
                    </CardDescription>
                  </CardHeader>
                </Card>
              )}

              {!submissionId && (
                <Recorder onSubmit={onSubmit} busy={submit.isPending} />
              )}

              {submit.isError && (
                <Card className="border-destructive/40">
                  <CardHeader>
                    <CardTitle className="text-base text-red-300">Upload failed</CardTitle>
                    <CardDescription>
                      {(submit.error as Error)?.message || 'Try recording again in a moment.'}
                    </CardDescription>
                  </CardHeader>
                </Card>
              )}

              {submissionId && (
                <>
                  <FeedbackCard
                    submission={stream.submission}
                    streamingText={stream.reviewText}
                    streamingTranscript={stream.transcript}
                    streamingStatus={stream.status}
                    errorMessage={stream.errorMessage}
                  />
                  {(stream.status === 'complete' || stream.status === 'failed') && (
                    <div className="flex justify-end">
                      <Button variant="outline" onClick={startOver}>
                        Record again
                      </Button>
                    </div>
                  )}
                </>
              )}

              <SubmissionHistoryList submissions={submissionHistory} hideId={submissionId} />
            </>
          )}
        </TabsContent>
      </Tabs>
    </article>
  )
}

// findInFlight picks the newest submission that's still in the review
// pipeline. The history list is already sorted newest-first by the server,
// so we just scan and return the first non-terminal row.
function findInFlight(rows: Submission[]): Submission | null {
  for (const r of rows) {
    if (r.status !== 'complete' && r.status !== 'failed') {
      return r
    }
  }
  return null
}
