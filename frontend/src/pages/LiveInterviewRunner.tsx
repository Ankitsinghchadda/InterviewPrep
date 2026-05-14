import { useEffect, useMemo, useState } from 'react'
import {
  CheckCircle2,
  Clock,
  Loader2,
  MessagesSquare,
  ScrollText,
  Square,
  X,
} from 'lucide-react'

import {
  useCompleteInterview,
  useNextLiveQuestion,
  usePollSubmission,
  useSubmitInterviewAnswer,
} from '@/hooks/queries'
import type { Interview, Question, Submission } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Recorder, type RecordingPayload } from '@/components/Recorder'
import { cn } from '@/lib/utils'

// LiveInterviewRunner drives an agentic, time-bounded interview: one question
// at a time, voice-only answers, dynamic follow-ups, and a server-side timer.
// Per-answer AI review is intentionally hidden — the user only sees the
// aggregate result on the post-interview screen (ResultView).
export function LiveInterviewRunner({
  interview,
  onChange,
}: {
  interview: Interview
  onChange: () => void
}) {
  const questions = interview.questions ?? []
  const submissions = interview.submissions ?? []
  const currentQuestion = questions[questions.length - 1]
  const turnIndex = Math.max(0, questions.length - 1)

  const endsAtMs = useMemo(() => {
    const startMs = new Date(interview.startedAt).getTime()
    return startMs + (interview.durationSeconds ?? 0) * 1000
  }, [interview.startedAt, interview.durationSeconds])

  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [])
  const remainingMs = Math.max(0, endsAtMs - now)
  const expired = remainingMs <= 0

  // Track the in-flight submission for the current turn. Seed with the most
  // recent submission so a page refresh mid-review doesn't lose state.
  const [submissionId, setSubmissionId] = useState<string | null>(() => {
    const last = submissions[submissions.length - 1]
    return last ? last.id : null
  })

  const submit = useSubmitInterviewAnswer(interview.id, currentQuestion?.id ?? '')
  const poll = usePollSubmission(submissionId)
  const next = useNextLiveQuestion(interview.id)
  const complete = useCompleteInterview(interview.id)

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [advancing, setAdvancing] = useState(false)
  const [wrapping, setWrapping] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const subStatus = poll.data?.status
  const subDone = subStatus === 'complete' || subStatus === 'failed'

  // When the latest submission's review finishes, auto-advance to the next
  // question (or wrap up). Guarded so it only fires once per submissionId.
  const [advancedFor, setAdvancedFor] = useState<string | null>(null)
  useEffect(() => {
    if (!submissionId || !subDone) return
    if (advancing || wrapping) return
    if (advancedFor === submissionId) return
    setAdvancedFor(submissionId)
    void advance()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [submissionId, subDone])

  async function advance() {
    setAdvancing(true)
    setErrorMsg(null)
    try {
      if (expired) {
        await finalize()
        return
      }
      const res = await next.mutateAsync()
      if (res.wrap) {
        await finalize()
        return
      }
      // Server appended a new question. Clear local submission state and let
      // the parent refetch the interview.
      setSubmissionId(null)
      onChange()
    } catch (err) {
      setErrorMsg((err as Error)?.message ?? 'Could not generate the next question.')
    } finally {
      setAdvancing(false)
    }
  }

  async function finalize() {
    setWrapping(true)
    try {
      await complete.mutateAsync()
      onChange()
    } catch (err) {
      setErrorMsg((err as Error)?.message ?? 'Could not finalize the interview.')
    } finally {
      setWrapping(false)
    }
  }

  async function onSubmit(payload: RecordingPayload) {
    if (!currentQuestion) return
    setErrorMsg(null)
    try {
      const sub = await submit.mutateAsync(payload)
      setSubmissionId(sub.id)
    } catch (err) {
      setErrorMsg((err as Error)?.message ?? 'Upload failed. Try again.')
    }
  }

  // Intro can run longer; later turns are tighter.
  const recorderMax = turnIndex === 0 ? 300 : 180

  const showRecorder = !submissionId && !advancing && !wrapping
  const showReviewing = Boolean(submissionId) && !subDone && !advancing && !wrapping
  const showThinking = advancing && !wrapping
  const showFinalizing = wrapping || complete.isPending

  return (
    <section className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-full bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/40">
            <MessagesSquare className="size-5" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Live Interview</h1>
            <p className="text-xs text-muted-foreground">
              Turn {turnIndex + 1} · feedback hidden until the interview ends
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <CountdownPill remainingMs={remainingMs} expired={expired} />
          <Button variant="outline" size="sm" onClick={() => setDrawerOpen((v) => !v)}>
            <ScrollText className="size-4" />
            {drawerOpen ? 'Hide transcript' : 'Transcript'}
          </Button>
          <EndButton onClick={() => finalize()} disabled={showFinalizing} />
        </div>
      </header>

      {expired && !wrapping && (
        <Card className="border-amber-500/40">
          <CardHeader>
            <CardTitle className="text-base text-amber-200">Time's up</CardTitle>
            <CardDescription>
              {showRecorder
                ? "Finish your current answer and submit — we'll wrap right after."
                : 'Wrapping the interview…'}
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      <div className={cn('grid gap-6', drawerOpen ? 'lg:grid-cols-[1fr_320px]' : '')}>
        <article className="space-y-5">
          {currentQuestion && (
            <Card>
              <CardHeader>
                <Badge variant="brand" className="w-fit">
                  Interviewer
                </Badge>
                <CardTitle className="mt-2 text-xl leading-snug">
                  {currentQuestion.title}
                </CardTitle>
                {currentQuestion.body && (
                  <CardDescription className="mt-1">{currentQuestion.body}</CardDescription>
                )}
              </CardHeader>
            </Card>
          )}

          {showRecorder && currentQuestion && (
            <Recorder
              key={currentQuestion.id}
              onSubmit={onSubmit}
              busy={submit.isPending}
              maxSeconds={recorderMax}
            />
          )}

          {showReviewing && <StatusCard text="Reviewing your answer…" />}
          {showThinking && <StatusCard text="The interviewer is thinking about the next question…" />}
          {showFinalizing && <StatusCard text="Wrapping up and aggregating feedback…" />}

          {errorMsg && (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
              <span>{errorMsg}</span>
              <Button variant="outline" size="sm" onClick={() => finalize()}>
                End interview
              </Button>
            </div>
          )}
        </article>

        {drawerOpen && (
          <aside className="space-y-3">
            <h2 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              Conversation so far
            </h2>
            <TranscriptDrawer
              questions={questions}
              submissions={submissions}
              activeQuestionId={currentQuestion?.id}
            />
          </aside>
        )}
      </div>
    </section>
  )
}

function StatusCard({ text }: { text: string }) {
  return (
    <Card>
      <CardContent className="flex items-center gap-2 py-6 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        {text}
      </CardContent>
    </Card>
  )
}

function CountdownPill({ remainingMs, expired }: { remainingMs: number; expired: boolean }) {
  const totalSec = Math.ceil(remainingMs / 1000)
  const mm = Math.floor(totalSec / 60)
    .toString()
    .padStart(2, '0')
  const ss = (totalSec % 60).toString().padStart(2, '0')
  const tone = expired
    ? 'border-red-500/50 bg-red-500/15 text-red-200'
    : totalSec < 120
      ? 'border-amber-500/50 bg-amber-500/15 text-amber-200'
      : 'border-brand-500/40 bg-brand-500/10 text-brand-100'
  return (
    <div
      className={cn(
        'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 font-mono text-sm tabular-nums',
        tone,
      )}
    >
      <Clock className="size-3.5" />
      {mm}:{ss}
    </div>
  )
}

function EndButton({ onClick, disabled }: { onClick: () => void; disabled: boolean }) {
  const [confirming, setConfirming] = useState(false)
  if (!confirming) {
    return (
      <Button variant="outline" size="sm" onClick={() => setConfirming(true)} disabled={disabled}>
        <Square className="size-3.5" /> End
      </Button>
    )
  }
  return (
    <div className="flex items-center gap-1">
      <Button
        variant="destructive"
        size="sm"
        onClick={() => {
          setConfirming(false)
          onClick()
        }}
        disabled={disabled}
      >
        <CheckCircle2 className="size-3.5" /> End now
      </Button>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setConfirming(false)}
        disabled={disabled}
      >
        <X className="size-3.5" />
      </Button>
    </div>
  )
}

function TranscriptDrawer({
  questions,
  submissions,
  activeQuestionId,
}: {
  questions: Question[]
  submissions: Submission[]
  activeQuestionId?: string
}) {
  const subByQ = new Map(submissions.map((s) => [s.questionId, s]))
  return (
    <div className="space-y-3">
      {questions.map((q, i) => {
        const sub = subByQ.get(q.id) ?? null
        const isActive = q.id === activeQuestionId
        return (
          <Card key={q.id} className={cn(isActive && 'ring-1 ring-brand-500/40')}>
            <CardHeader className="pb-2">
              <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
                Turn {i + 1}
              </div>
              <CardTitle className="text-sm leading-snug">{q.title}</CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              {sub?.transcript ? (
                <p className="text-xs leading-relaxed text-foreground/80">{sub.transcript}</p>
              ) : (
                <p className="text-xs italic text-muted-foreground">No answer yet.</p>
              )}
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
