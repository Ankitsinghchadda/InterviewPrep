import { useMemo, useState } from 'react'
import { Bot, ChevronDown, ChevronUp, FileText, History, Sparkles } from 'lucide-react'

import type { Submission } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { FeedbackCard } from '@/components/FeedbackCard'
import { cn } from '@/lib/utils'

type SortMode = 'recent' | 'best'

interface SubmissionHistoryListProps {
  submissions: Submission[]
  /** When set, this submission is the one currently being streamed and is */
  /** rendered separately above the history. Hide it from the list to avoid */
  /** showing the same row twice. */
  hideId?: string | null
}

// SubmissionHistoryList renders the user's past attempts on a single question.
// Each row collapses to a one-line summary; clicking expands the full
// FeedbackCard (strengths, improvements, transcript). Default sort is recency;
// "Best" sorts by score desc with nulls last.
export function SubmissionHistoryList({ submissions, hideId }: SubmissionHistoryListProps) {
  const [sort, setSort] = useState<SortMode>('recent')
  const [expanded, setExpanded] = useState<string | null>(null)

  const rows = useMemo(() => {
    const filtered = submissions.filter((s) => s.id !== hideId)
    if (sort === 'best') {
      return [...filtered].sort((a, b) => {
        const sa = a.score ?? -1
        const sb = b.score ?? -1
        return sb - sa
      })
    }
    return [...filtered].sort(
      (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
    )
  }, [submissions, hideId, sort])

  if (rows.length === 0) {
    return null
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-3 space-y-0">
        <div className="flex items-center gap-2">
          <History className="size-4 text-muted-foreground" />
          <CardTitle className="text-base">Your past attempts</CardTitle>
          <Badge variant="outline" className="text-muted-foreground">
            {rows.length}
          </Badge>
        </div>
        <div className="flex items-center gap-1 rounded-md border border-border/60 p-0.5 text-xs">
          <SortButton active={sort === 'recent'} onClick={() => setSort('recent')}>
            Recent
          </SortButton>
          <SortButton active={sort === 'best'} onClick={() => setSort('best')}>
            Best
          </SortButton>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {rows.map((s) => (
          <HistoryRow
            key={s.id}
            submission={s}
            isExpanded={expanded === s.id}
            onToggle={() => setExpanded((id) => (id === s.id ? null : s.id))}
          />
        ))}
      </CardContent>
    </Card>
  )
}

function HistoryRow({
  submission,
  isExpanded,
  onToggle,
}: {
  submission: Submission
  isExpanded: boolean
  onToggle: () => void
}) {
  const [showReview, setShowReview] = useState(false)
  const score = submission.score != null ? Math.round(submission.score) : null
  const date = new Date(submission.createdAt)
  const dateLabel = date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
  const status = submission.status
  const isTerminal = status === 'complete' || status === 'failed'
  const transcript = submission.transcript?.trim() ?? ''
  const hasReview =
    status === 'complete' &&
    (submission.feedback ||
      (submission.strengths && submission.strengths.length > 0) ||
      (submission.improvements && submission.improvements.length > 0))

  return (
    <div className="rounded-md border border-border/60 bg-background/40">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors hover:bg-accent/30"
      >
        <div className="flex min-w-0 items-center gap-3">
          <ScoreChip score={score} />
          <div className="min-w-0">
            <p className="text-sm text-foreground">{dateLabel}</p>
            <p className="truncate text-xs text-muted-foreground">
              {status === 'complete'
                ? answerSnippet(submission)
                : status === 'failed'
                  ? submission.errorMessage || 'Review failed.'
                  : 'In progress…'}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {!isTerminal && (
            <Badge variant="outline" className="text-muted-foreground">
              {status}
            </Badge>
          )}
          {isExpanded ? (
            <ChevronUp className="size-4 text-muted-foreground" />
          ) : (
            <ChevronDown className="size-4 text-muted-foreground" />
          )}
        </div>
      </button>
      {isExpanded && (
        <div className="space-y-3 border-t border-border/60 p-3">
          {status === 'complete' ? (
            <>
              <div className="rounded-md border border-border/50 bg-background/60 p-4">
                <div className="mb-2 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  <FileText className="size-3.5" /> Your answer
                </div>
                {transcript ? (
                  <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">
                    {transcript}
                  </p>
                ) : (
                  <p className="text-sm italic text-muted-foreground">
                    No transcript captured for this attempt.
                  </p>
                )}
              </div>

              {hasReview && (
                <>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => setShowReview((v) => !v)}
                    className="h-8 gap-1.5 text-xs"
                  >
                    {showReview ? (
                      <>
                        <ChevronUp className="size-3.5" />
                        Hide AI review
                      </>
                    ) : (
                      <>
                        <Sparkles className="size-3.5 text-brand-300" />
                        Show AI review
                      </>
                    )}
                  </Button>

                  {showReview && (
                    <ReviewPanel submission={submission} score={score} />
                  )}
                </>
              )}
            </>
          ) : (
            <FeedbackCard
              submission={submission}
              streamingStatus={mapStatus(status)}
              errorMessage={submission.errorMessage}
            />
          )}
        </div>
      )}
    </div>
  )
}

function ReviewPanel({
  submission,
  score,
}: {
  submission: Submission
  score: number | null
}) {
  const tone = score != null ? scoreTone(score) : null
  return (
    <div className="overflow-hidden rounded-md border border-border/50 bg-background/60">
      <div className="flex items-start justify-between gap-3 p-4">
        <div className="min-w-0">
          <div className="mb-1 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-brand-300">
            <Bot className="size-3.5" /> AI review
          </div>
          {submission.feedback && (
            <p className="text-sm leading-relaxed text-foreground/90">
              {submission.feedback}
            </p>
          )}
        </div>
        {score != null && tone && (
          <div className="flex shrink-0 items-center gap-2">
            <div
              className={cn(
                'grid size-12 place-items-center rounded-lg font-mono text-lg font-bold ring-1 ring-inset',
                tone.ring,
                tone.bg,
                tone.text,
              )}
              aria-label={`Score ${score} out of 100`}
            >
              {score}
            </div>
            <div className="text-[10px] uppercase tracking-wider text-muted-foreground">
              / 100
            </div>
          </div>
        )}
      </div>

      {((submission.strengths?.length ?? 0) > 0 ||
        (submission.improvements?.length ?? 0) > 0) && (
        <div className="grid gap-4 border-t border-border/50 p-4 md:grid-cols-2">
          {(submission.strengths?.length ?? 0) > 0 && (
            <ReviewList
              title="Strengths"
              tone="emerald"
              items={submission.strengths ?? []}
            />
          )}
          {(submission.improvements?.length ?? 0) > 0 && (
            <ReviewList
              title="Improvements"
              tone="amber"
              items={submission.improvements ?? []}
            />
          )}
        </div>
      )}
    </div>
  )
}

function ReviewList({
  title,
  tone,
  items,
}: {
  title: string
  tone: 'emerald' | 'amber'
  items: string[]
}) {
  const dot = tone === 'emerald' ? 'bg-emerald-400' : 'bg-amber-400'
  const badgeVariant = tone === 'emerald' ? 'success' : 'brand'
  return (
    <div>
      <div className="mb-2">
        <Badge variant={badgeVariant}>{title}</Badge>
      </div>
      <ul className="space-y-1.5">
        {items.map((it, i) => (
          <li key={i} className="flex gap-2 text-sm leading-relaxed">
            <span className={cn('mt-1.5 size-1.5 shrink-0 rounded-full', dot)} />
            <span>{it}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function SortButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      onClick={onClick}
      className={cn(
        'h-7 rounded-sm px-2 text-xs',
        active && 'bg-accent text-foreground',
      )}
    >
      {children}
    </Button>
  )
}

function ScoreChip({ score }: { score: number | null }) {
  if (score == null) {
    return (
      <span className="grid size-10 shrink-0 place-items-center rounded-md border border-border/60 text-xs text-muted-foreground">
        —
      </span>
    )
  }
  const tone = scoreTone(score)
  return (
    <span
      className={cn(
        'grid size-10 shrink-0 place-items-center rounded-md font-mono text-sm font-semibold ring-1 ring-inset',
        tone.ring,
        tone.bg,
        tone.text,
      )}
      aria-label={`Score ${score}`}
    >
      {score}
    </span>
  )
}

function answerSnippet(s: Submission): string {
  if (s.transcript) {
    return s.transcript.length > 120 ? s.transcript.slice(0, 117) + '…' : s.transcript
  }
  if (s.feedback) {
    return s.feedback.length > 120 ? s.feedback.slice(0, 117) + '…' : s.feedback
  }
  return 'Tap to view your answer.'
}

function mapStatus(s: Submission['status']) {
  switch (s) {
    case 'complete':
      return 'complete' as const
    case 'failed':
      return 'failed' as const
    case 'transcribing':
      return 'transcribing' as const
    case 'reviewing':
      return 'reviewing' as const
    default:
      return 'connecting' as const
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
