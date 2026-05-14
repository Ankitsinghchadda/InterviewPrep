import { Bot, Check, FileText, Loader2, TriangleAlert, Waves } from 'lucide-react'

import type { Submission, SubmissionStatus } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

export type FeedbackStreamStatus =
  | 'connecting'
  | 'transcribing'
  | 'reviewing'
  | 'complete'
  | 'failed'

interface FeedbackCardProps {
  /** Persisted submission row. Null while we wait for the first stream event. */
  submission: Submission | null
  /** Running concatenation of review_token chunks during streaming. */
  streamingText?: string
  /** Server-confirmed transcript (after STT or accepted client transcript). */
  streamingTranscript?: string
  /** Derived stream status. Falls back to the submission's status when omitted. */
  streamingStatus?: FeedbackStreamStatus
  /** Terminal error from the stream. */
  errorMessage?: string | null
}

export function FeedbackCard({
  submission,
  streamingText,
  streamingTranscript,
  streamingStatus,
  errorMessage,
}: FeedbackCardProps) {
  const status: FeedbackStreamStatus = streamingStatus ?? mapStatus(submission?.status)

  if (status === 'failed') {
    return (
      <Card className="border-destructive/40">
        <CardHeader>
          <div className="flex items-center gap-2 text-red-300">
            <TriangleAlert className="size-4" />
            <CardTitle className="text-base">Review failed</CardTitle>
          </div>
          <CardDescription>
            {errorMessage ||
              submission?.errorMessage ||
              'Something went wrong while processing this answer. Try recording again.'}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const isComplete = status === 'complete' && submission
  const transcript = streamingTranscript || submission?.transcript || ''

  if (!isComplete) {
    return (
      <StreamingCard
        status={status}
        transcript={transcript}
        streamingText={streamingText ?? ''}
      />
    )
  }

  // Terminal complete state — render the structured review layout.
  const score = Math.round(submission!.score ?? 0)
  const tone = scoreTone(score)

  return (
    <Card className="overflow-hidden">
      <div className="grid gap-0 md:grid-cols-[1fr_auto]">
        <CardHeader className="md:pr-2">
          <div className="flex items-center gap-2">
            <Bot className="size-4 text-brand-300" />
            <CardTitle className="text-base">AI review</CardTitle>
          </div>
          {submission!.feedback && (
            <CardDescription className="mt-1 text-sm leading-relaxed text-foreground/90">
              {submission!.feedback}
            </CardDescription>
          )}
        </CardHeader>
        <div className="flex items-center gap-3 px-6 pb-2 pt-4 md:py-6">
          <div
            className={cn(
              'grid size-16 place-items-center rounded-2xl font-mono text-2xl font-bold ring-1 ring-inset',
              tone.ring,
              tone.bg,
              tone.text,
            )}
            aria-label={`Score ${score} out of 100`}
          >
            {score}
          </div>
          <div className="text-xs uppercase tracking-wider text-muted-foreground">/ 100</div>
        </div>
      </div>

      <CardContent className="grid gap-6 border-t border-border/60 pt-5 md:grid-cols-2">
        <FeedbackList
          title="Strengths"
          tone="emerald"
          items={submission!.strengths ?? []}
          icon={Check}
          empty="No standout strengths captured."
        />
        <FeedbackList
          title="Improvements"
          tone="amber"
          items={submission!.improvements ?? []}
          icon={Waves}
          empty="No improvements suggested."
        />
      </CardContent>

      {submission!.transcript && (
        <CardContent className="border-t border-border/60 pt-5">
          <h4 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Transcript
          </h4>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/80">
            {submission!.transcript}
          </p>
        </CardContent>
      )}
    </Card>
  )
}

// StreamingCard renders the in-flight view: progress bar, the confirmed
// transcript (if any), and the running review text with a blinking cursor.
function StreamingCard({
  status,
  transcript,
  streamingText,
}: {
  status: FeedbackStreamStatus
  transcript: string
  streamingText: string
}) {
  const label = {
    connecting: 'Connecting…',
    transcribing: 'Transcribing your answer…',
    reviewing: 'Reviewing your answer…',
    complete: 'Complete',
    failed: 'Failed',
  }[status]
  const sub = {
    connecting: 'Hooking up to the review stream.',
    transcribing: 'Converting your speech to text on the server.',
    reviewing: 'The AI interviewer is grading your response.',
    complete: '',
    failed: '',
  }[status]
  const widthCls = {
    connecting: 'w-1/6',
    transcribing: 'w-2/6 animate-pulse',
    reviewing: 'w-4/6 animate-pulse',
    complete: 'w-full',
    failed: 'w-full',
  }[status]

  return (
    <Card className="overflow-hidden">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Loader2 className="size-4 animate-spin text-brand-300" />
          <CardTitle className="text-base">{label}</CardTitle>
        </div>
        <CardDescription>{sub}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
          <div
            className={cn(
              'h-full bg-gradient-to-r from-brand-400 to-fuchsia-400 transition-all',
              widthCls,
            )}
          />
        </div>

        {transcript && (
          <div className="rounded-md border border-border/40 bg-background/30 p-3">
            <div className="mb-1 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              <FileText className="size-3.5" /> Transcript
            </div>
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/85">
              {transcript}
            </p>
          </div>
        )}

        {streamingText && (
          <div className="rounded-md border border-brand-500/30 bg-brand-500/5 p-3">
            <div className="mb-1 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-brand-300">
              <Bot className="size-3.5" /> Live review
            </div>
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">
              {streamingText}
              <span className="ml-0.5 inline-block h-3.5 w-0.5 animate-pulse bg-brand-300 align-middle" />
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function FeedbackList({
  title,
  tone,
  items,
  icon: Icon,
  empty,
}: {
  title: string
  tone: 'emerald' | 'amber'
  items: string[]
  icon: React.ComponentType<{ className?: string }>
  empty: string
}) {
  const toneClass = tone === 'emerald' ? 'text-emerald-400' : 'text-amber-400'
  const badgeVariant = tone === 'emerald' ? 'success' : 'brand'
  return (
    <div>
      <div className="mb-3 flex items-center gap-2">
        <Badge variant={badgeVariant}>{title}</Badge>
      </div>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{empty}</p>
      ) : (
        <ul className="space-y-2">
          {items.map((it, i) => (
            <li key={i} className="flex gap-2 text-sm leading-relaxed">
              <Icon className={cn('mt-0.5 size-4 shrink-0', toneClass)} />
              <span>{it}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

// mapStatus collapses the wider SubmissionStatus union from the DB into the
// narrower FeedbackStreamStatus the UI cares about.
function mapStatus(s: SubmissionStatus | undefined): FeedbackStreamStatus {
  switch (s) {
    case 'pending':
      return 'connecting'
    case 'transcribing':
    case 'reviewing':
    case 'complete':
    case 'failed':
      return s
    default:
      return 'connecting'
  }
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
