import { ArrowDown, ArrowRight, ArrowUp, Flame, ListChecks, Mic, Target } from 'lucide-react'

import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import type { StatsOverview } from '@/types'

interface Props {
  stats: StatsOverview | null | undefined
  loading: boolean
}

export function StatsHero({ stats, loading }: Props) {
  if (loading) return <StatsHeroSkeleton />

  const v = stats?.volume
  const s = stats?.scoring
  const streak = stats?.streak
  const goal = stats?.goalAlignment

  const avg = s?.averageScore
  const delta =
    avg != null && s?.averagePrior30 != null
      ? Math.round(avg - s.averagePrior30)
      : null

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <HeroTile
        icon={<ListChecks className="size-4" />}
        label="Questions practiced"
        value={v ? formatBigNumber(v.totalSubmissions) : '—'}
        hint={
          v && v.submissionsLast30Days > 0
            ? `${v.submissionsLast30Days} in last 30 days`
            : 'Start practicing to track progress'
        }
      />
      <HeroTile
        icon={<Target className="size-4" />}
        label="Avg score"
        value={avg != null ? `${Math.round(avg)}` : '—'}
        valueSuffix={avg != null ? <span className="text-sm text-muted-foreground">/100</span> : null}
        hint={
          delta == null
            ? 'After a few submissions'
            : delta === 0
              ? 'Steady vs prior 30 days'
              : delta > 0
                ? `+${delta} vs prior 30 days`
                : `${delta} vs prior 30 days`
        }
        hintIcon={
          delta == null
            ? undefined
            : delta > 0
              ? <ArrowUp className="size-3 text-emerald-300" />
              : delta < 0
                ? <ArrowDown className="size-3 text-red-300" />
                : <ArrowRight className="size-3 text-muted-foreground" />
        }
      />
      <HeroTile
        icon={<Mic className="size-4" />}
        label="Interviews completed"
        value={v ? `${v.interviewsCompleted}` : '—'}
        hint={
          v && v.interviewsStarted > v.interviewsCompleted
            ? `${v.interviewsStarted - v.interviewsCompleted} in progress`
            : 'Mock interviews you finished'
        }
      />
      <StreakTile streak={streak} alignment={goal} />
    </div>
  )
}

function HeroTile({
  icon,
  label,
  value,
  valueSuffix,
  hint,
  hintIcon,
}: {
  icon: React.ReactNode
  label: string
  value: string
  valueSuffix?: React.ReactNode
  hint: string
  hintIcon?: React.ReactNode
}) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-1.5 p-4">
        <div className="flex items-center gap-2 text-xs uppercase tracking-wide text-muted-foreground">
          <span className="grid size-6 place-items-center rounded-md bg-muted text-foreground/70">
            {icon}
          </span>
          {label}
        </div>
        <div className="flex items-baseline gap-1">
          <span className="font-mono text-2xl font-bold tracking-tight">{value}</span>
          {valueSuffix}
        </div>
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          {hintIcon}
          <span>{hint}</span>
        </div>
      </CardContent>
    </Card>
  )
}

function StreakTile({
  streak,
  alignment,
}: {
  streak: StatsOverview['streak'] | undefined
  alignment: StatsOverview['goalAlignment'] | undefined
}) {
  const days = streak?.current ?? 0
  const hasGoal = Boolean(alignment?.targetRole)
  const pct = alignment ? Math.round(alignment.alignmentPercent) : 0

  return (
    <Card>
      <CardContent className="flex flex-col gap-1.5 p-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs uppercase tracking-wide text-muted-foreground">
            <span className="grid size-6 place-items-center rounded-md bg-muted text-foreground/70">
              <Flame className="size-4" />
            </span>
            Streak
          </div>
          {hasGoal && (
            <GoalRing percent={pct} />
          )}
        </div>
        <div className="flex items-baseline gap-1">
          <span className={cn('font-mono text-2xl font-bold tracking-tight', days > 0 && 'text-emerald-300')}>
            {days}
          </span>
          <span className="text-sm text-muted-foreground">{days === 1 ? 'day' : 'days'}</span>
        </div>
        <div className="text-xs text-muted-foreground">
          {streak?.practicedToday
            ? 'Practiced today'
            : streak?.longest
              ? `Longest: ${streak.longest} ${streak.longest === 1 ? 'day' : 'days'}`
              : 'Practice today to start a streak'}
        </div>
      </CardContent>
    </Card>
  )
}

function GoalRing({ percent }: { percent: number }) {
  const pct = Math.max(0, Math.min(100, percent))
  const size = 32
  const stroke = 4
  const r = (size - stroke) / 2
  const c = 2 * Math.PI * r
  const offset = c - (pct / 100) * c

  return (
    <div className="relative" title={`${pct}% of practice aligned with your goal`}>
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="currentColor"
          strokeWidth={stroke}
          fill="none"
          className="text-muted/40"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="currentColor"
          strokeWidth={stroke}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={offset}
          className="text-brand-300"
        />
      </svg>
      <span className="pointer-events-none absolute inset-0 grid place-items-center font-mono text-[10px] font-semibold">
        {pct}
      </span>
    </div>
  )
}

function formatBigNumber(n: number) {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return `${n}`
}

export function StatsHeroSkeleton() {
  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="h-[96px] animate-pulse rounded-xl border border-border/40 bg-card/40" />
      ))}
    </div>
  )
}
