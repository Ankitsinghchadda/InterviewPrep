import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Flag,
  Loader2,
  Mic,
  Sparkles,
  TriangleAlert,
} from 'lucide-react'

import {
  useCompleteInterview,
  useInterview,
  useStreamSubmission,
  useSubmitInterviewAnswer,
} from '@/hooks/queries'
import type { Interview, Question, Submission } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Recorder, type RecordingPayload } from '@/components/Recorder'
import { FeedbackCard } from '@/components/FeedbackCard'
import { LiveInterviewRunner } from '@/pages/LiveInterviewRunner'
import { cn } from '@/lib/utils'

export function InterviewRunner() {
  const { id } = useParams<{ id: string }>()
  const { data: interview, isLoading, error, refetch } = useInterview(id)

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading interview…
      </div>
    )
  }

  if (error || !interview) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Interview not found</CardTitle>
          <CardDescription>It may have been removed or you don’t have access.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant="outline">
            <Link to="/interview">
              <ArrowLeft className="size-4" /> Start a new interview
            </Link>
          </Button>
        </CardContent>
      </Card>
    )
  }

  if (interview.status === 'completed') {
    return <ResultView interview={interview} />
  }

  if (interview.mode === 'live') {
    return <LiveInterviewRunner interview={interview} onChange={() => refetch()} />
  }

  return <InProgressView interview={interview} onChange={() => refetch()} />
}

// ---- In-progress ---------------------------------------------------------

function InProgressView({ interview, onChange }: { interview: Interview; onChange: () => void }) {
  const questions = interview.questions ?? []
  const submissions = interview.submissions ?? []

  // The current question is the first one without a submission on initial load.
  // After mount it's driven purely by the Next button so we don't fight the user.
  const [currentIdx, setCurrentIdx] = useState(() => {
    const submitted = new Set(submissions.map((s) => s.questionId))
    const idx = questions.findIndex((q) => !submitted.has(q.id))
    return idx >= 0 ? idx : Math.max(0, questions.length - 1)
  })

  const currentQuestion = questions[currentIdx]
  const currentSubmission = currentQuestion
    ? submissions.find((s) => s.questionId === currentQuestion.id) ?? null
    : null

  const total = questions.length
  const answered = submissions.length

  const allAnswered = answered === total && submissions.every((s) => s.status === 'complete' || s.status === 'failed')

  return (
    <section className="space-y-6">
      <header className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Mock Interview</h1>
            <p className="text-sm text-muted-foreground">
              Question {Math.min(currentIdx + 1, total)} of {total}
            </p>
          </div>
          <FinishButton interviewId={interview.id} disabled={!allAnswered} />
        </div>
        <Progress total={total} answered={answered} currentIdx={currentIdx} />
      </header>

      {currentQuestion && (
        <QuestionStep
          key={currentQuestion.id}
          interviewId={interview.id}
          question={currentQuestion}
          existingSubmission={currentSubmission}
          isLast={currentIdx === total - 1}
          onAdvance={() => {
            if (currentIdx < total - 1) setCurrentIdx(currentIdx + 1)
            onChange()
          }}
        />
      )}
    </section>
  )
}

function Progress({
  total,
  answered,
  currentIdx,
}: {
  total: number
  answered: number
  currentIdx: number
}) {
  return (
    <div className="space-y-2">
      <div className="flex gap-1">
        {Array.from({ length: total }).map((_, i) => (
          <span
            key={i}
            className={cn(
              'h-1.5 flex-1 rounded-full transition-colors',
              i < answered
                ? 'bg-emerald-500/70'
                : i === currentIdx
                  ? 'bg-brand-400'
                  : 'bg-muted',
            )}
          />
        ))}
      </div>
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{answered} answered</span>
        <span>{total - answered} remaining</span>
      </div>
    </div>
  )
}

function QuestionStep({
  interviewId,
  question,
  existingSubmission,
  isLast,
  onAdvance,
}: {
  interviewId: string
  question: Question
  existingSubmission: Submission | null
  isLast: boolean
  onAdvance: () => void
}) {
  const submit = useSubmitInterviewAnswer(interviewId, question.id)
  // Parent passes `key={question.id}` so this component remounts on question
  // change — no effect-based reset needed.
  const [submissionId, setSubmissionId] = useState<string | null>(
    existingSubmission?.id ?? null,
  )
  const stream = useStreamSubmission(submissionId)

  const onSubmit = async (payload: RecordingPayload) => {
    try {
      const sub = await submit.mutateAsync(payload)
      setSubmissionId(sub.id)
    } catch (err) {
      console.error('submission failed', err)
    }
  }

  const reviewDone = stream.status === 'complete' || stream.status === 'failed'

  return (
    <article className="space-y-5">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="brand">{question.difficulty}</Badge>
            {question.categories.slice(0, 6).map((slug) => (
              <Badge key={slug} variant="outline" className="text-muted-foreground">
                {slug}
              </Badge>
            ))}
          </div>
          <CardTitle className="mt-2 text-xl">{question.title}</CardTitle>
          {question.body && (
            <CardDescription className="mt-1">{question.body}</CardDescription>
          )}
        </CardHeader>
        {question.answer && (
          <CardContent>
            <details className="group">
              <summary className="flex cursor-pointer select-none items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground">
                <BookOpen className="size-3.5" />
                Peek at the reference answer (only after recording)
              </summary>
              <p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-foreground/85">
                {question.answer}
              </p>
            </details>
          </CardContent>
        )}
      </Card>

      {!submissionId && (
        <Recorder onSubmit={onSubmit} busy={submit.isPending} />
      )}

      {submit.isError && (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
          {(submit.error as Error)?.message || 'Upload failed. Try again.'}
        </p>
      )}

      {submissionId && (
        <FeedbackCard
          submission={stream.submission}
          streamingText={stream.reviewText}
          streamingTranscript={stream.transcript}
          streamingStatus={stream.status}
          errorMessage={stream.errorMessage}
        />
      )}

      {submissionId && (
        <div className="flex justify-end">
          <Button
            variant={reviewDone ? 'brand' : 'outline'}
            size="lg"
            onClick={onAdvance}
            disabled={!reviewDone}
          >
            {isLast ? (
              <>
                <Flag className="size-4" /> Go to finish
              </>
            ) : (
              <>
                Next question <ArrowRight className="size-4" />
              </>
            )}
          </Button>
        </div>
      )}
    </article>
  )
}

function FinishButton({ interviewId, disabled }: { interviewId: string; disabled: boolean }) {
  const complete = useCompleteInterview(interviewId)
  const onClick = () => {
    complete.mutate()
  }
  return (
    <div className="flex flex-col items-end gap-1">
      <Button
        variant="brand"
        size="lg"
        onClick={onClick}
        disabled={disabled || complete.isPending}
      >
        {complete.isPending ? (
          <>
            <Loader2 className="size-4 animate-spin" /> Aggregating…
          </>
        ) : (
          <>
            <CheckCircle2 className="size-4" /> Finish interview
          </>
        )}
      </Button>
      {complete.isError && (
        <span className="text-xs text-red-300">
          {(complete.error as Error)?.message || 'Could not finalize. Try again.'}
        </span>
      )}
    </div>
  )
}

// ---- Result view ---------------------------------------------------------

function ResultView({ interview }: { interview: Interview }) {
  const questions = interview.questions ?? []
  const submissions = interview.submissions ?? []
  const subByQ = new Map(submissions.map((s) => [s.questionId, s]))

  const overall = Math.round(interview.score ?? 0)
  const tone = scoreTone(overall)

  return (
    <section className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Interview complete
          </p>
          <h1 className="mt-1 text-2xl font-bold tracking-tight sm:text-3xl">Your result</h1>
        </div>
        <Button asChild variant="outline">
          <Link to="/interview">Take another</Link>
        </Button>
      </header>

      <Card className="overflow-hidden">
        <div className="grid gap-0 md:grid-cols-[1fr_auto]">
          <CardHeader className="md:pr-2">
            <div className="flex items-center gap-2">
              <Sparkles className="size-4 text-brand-300" />
              <CardTitle className="text-base">Final review</CardTitle>
            </div>
            {interview.summary && (
              <CardDescription className="mt-1 text-sm leading-relaxed text-foreground/90">
                {interview.summary}
              </CardDescription>
            )}
          </CardHeader>
          <div className="flex items-center gap-3 px-6 pb-2 pt-4 md:py-6">
            <div
              className={cn(
                'grid size-20 place-items-center rounded-2xl font-mono text-3xl font-bold ring-1 ring-inset',
                tone.ring,
                tone.bg,
                tone.text,
              )}
              aria-label={`Overall ${overall} out of 100`}
            >
              {overall}
            </div>
            <div className="text-xs uppercase tracking-wider text-muted-foreground">/ 100</div>
          </div>
        </div>
      </Card>

      <div className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
          Per-question breakdown
        </h2>
        {questions.map((q, i) => {
          const sub = subByQ.get(q.id) ?? null
          return <PerQuestionRow key={q.id} index={i + 1} question={q} submission={sub} />
        })}
      </div>
    </section>
  )
}

function PerQuestionRow({
  index,
  question,
  submission,
}: {
  index: number
  question: Question
  submission: Submission | null
}) {
  const score = submission?.score == null ? null : Math.round(submission.score)
  const failed = submission?.status === 'failed'

  return (
    <Card>
      <CardHeader className="space-y-2">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex-1">
            <div className="text-xs text-muted-foreground">Question {index}</div>
            <CardTitle className="text-base">{question.title}</CardTitle>
          </div>
          <ScorePill score={score} failed={failed} />
        </div>
      </CardHeader>
      {submission?.feedback && (
        <CardContent className="pt-0">
          <p className="text-sm leading-relaxed text-foreground/85">{submission.feedback}</p>
          {(submission.strengths?.length || submission.improvements?.length) ? (
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              {submission.strengths && submission.strengths.length > 0 && (
                <div>
                  <div className="mb-1 text-xs font-medium uppercase tracking-wider text-emerald-300">
                    Strengths
                  </div>
                  <ul className="space-y-1 text-sm text-muted-foreground">
                    {submission.strengths.map((s, i) => (
                      <li key={i}>• {s}</li>
                    ))}
                  </ul>
                </div>
              )}
              {submission.improvements && submission.improvements.length > 0 && (
                <div>
                  <div className="mb-1 text-xs font-medium uppercase tracking-wider text-amber-300">
                    Improvements
                  </div>
                  <ul className="space-y-1 text-sm text-muted-foreground">
                    {submission.improvements.map((s, i) => (
                      <li key={i}>• {s}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          ) : null}
        </CardContent>
      )}
      {!submission && (
        <CardContent className="pt-0">
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Mic className="size-3.5" /> No answer recorded.
          </p>
        </CardContent>
      )}
    </Card>
  )
}

function ScorePill({ score, failed }: { score: number | null; failed: boolean }) {
  if (failed) {
    return (
      <Badge variant="destructive" className="gap-1">
        <TriangleAlert className="size-3" /> failed
      </Badge>
    )
  }
  if (score == null) {
    return <Badge variant="outline" className="text-muted-foreground">no answer</Badge>
  }
  const tone = scoreTone(score)
  return (
    <div
      className={cn(
        'flex h-12 w-16 flex-col items-center justify-center rounded-md font-mono text-lg font-bold ring-1 ring-inset',
        tone.ring,
        tone.bg,
        tone.text,
      )}
    >
      <span>{score}</span>
      <span className="text-[10px] uppercase tracking-wider opacity-60">/100</span>
    </div>
  )
}

function scoreTone(score: number) {
  if (score >= 80) {
    return {
      ring: 'ring-emerald-500/40',
      bg: 'bg-emerald-500/15',
      text: 'text-emerald-300',
    }
  }
  if (score >= 55) {
    return {
      ring: 'ring-brand-500/40',
      bg: 'bg-brand-500/15',
      text: 'text-brand-200',
    }
  }
  return {
    ring: 'ring-red-500/40',
    bg: 'bg-red-500/15',
    text: 'text-red-200',
  }
}
