import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  ArrowRight,
  CheckCircle2,
  ListChecks,
  Mic,
  PlayCircle,
  Plus,
  Sparkles,
  TriangleAlert,
  UserCircle2,
} from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import {
  useMyInterviews,
  useProfile,
  useSmartRecommendations,
  useStatsOverview,
} from '@/hooks/queries'
import type { Interview } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { CategoryPerformance } from '@/components/dashboard/CategoryPerformance'
import { DifficultyMix } from '@/components/dashboard/DifficultyMix'
import { ScoreTrendChart } from '@/components/dashboard/ScoreTrendChart'
import { SmartRecommendations } from '@/components/dashboard/SmartRecommendations'
import { StatsHero } from '@/components/dashboard/StatsHero'
import { ThemesList } from '@/components/dashboard/ThemesList'

const QUICK_ACTIONS = [
  {
    icon: ListChecks,
    title: 'Browse topics',
    body: 'Pick a role or technology and explore curated questions.',
    to: '/topics',
    cta: 'Open topics',
  },
  {
    icon: Mic,
    title: 'Practice a question',
    body: 'Record an answer to a single question and get instant feedback.',
    to: '/questions',
    cta: 'See questions',
  },
  {
    icon: PlayCircle,
    title: 'Run a mock interview',
    body: 'Choose categories or use Adaptive mode to match your profile.',
    to: '/interview',
    cta: 'Start interview',
  },
]

export function Home() {
  const { user } = useAuth()
  const { data: profile } = useProfile()
  const { data: stats, isLoading: statsLoading } = useStatsOverview()
  const { data: recs, isLoading: recsLoading } = useSmartRecommendations()
  const { data: interviews } = useMyInterviews()

  const firstName = user?.name?.split(' ')[0] || 'there'
  const onboarded = Boolean(profile?.onboardedAt)
  const allInterviews = interviews ?? []

  return (
    <section className="space-y-10">
      <header>
        <p className="text-sm text-muted-foreground">Welcome back</p>
        <h1 className="mt-1 text-2xl font-bold tracking-tight sm:text-3xl md:text-4xl">
          Hey {firstName} — {onboarded ? 'here’s your progress.' : 'let’s tailor this to you.'}
        </h1>
      </header>

      {!onboarded && <OnboardingNudge />}

      <StatsHero stats={stats} loading={statsLoading} />

      <div className="grid gap-6 md:grid-cols-3">
        {QUICK_ACTIONS.map(({ icon: Icon, title, body, to, cta }) => (
          <Card key={title} className="flex flex-col">
            <CardHeader>
              <div className="mb-2 inline-flex size-9 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                <Icon className="size-5" />
              </div>
              <CardTitle>{title}</CardTitle>
              <CardDescription>{body}</CardDescription>
            </CardHeader>
            <CardContent className="mt-auto pt-0">
              <Button asChild variant="outline" size="sm">
                <Link to={to}>
                  {cta} <ArrowRight className="size-4" />
                </Link>
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      <SmartRecommendations recs={recs} loading={recsLoading} />

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <ScoreTrendChart trend={stats?.trend ?? []} loading={statsLoading} />
        </div>
        <DifficultyMix buckets={stats?.difficultyDistribution} loading={statsLoading} />
      </div>

      <CategoryPerformance data={stats?.categories} loading={statsLoading} />

      <div className="grid gap-6 md:grid-cols-2">
        <ThemesList variant="strengths" themes={stats?.themes.strengths} loading={statsLoading} />
        <ThemesList variant="improvements" themes={stats?.themes.improvements} loading={statsLoading} />
      </div>

      <RecentInterviews interviews={allInterviews} />
    </section>
  )
}

const INTERVIEW_PAGE_SIZE = 5

function RecentInterviews({ interviews }: { interviews: Interview[] }) {
  const [visible, setVisible] = useState(INTERVIEW_PAGE_SIZE)
  const shown = interviews.slice(0, visible)
  const hasMore = interviews.length > visible

  return (
    <section className="space-y-4">
      <div className="flex items-baseline justify-between">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">Recent interviews</h2>
          {interviews.length > 0 && (
            <p className="text-sm text-muted-foreground">
              Showing {shown.length} of {interviews.length}
            </p>
          )}
        </div>
        {interviews.length > 0 && (
          <Button asChild variant="brand" size="sm">
            <Link to="/interview">
              <Plus className="size-4" /> New interview
            </Link>
          </Button>
        )}
      </div>
      {interviews.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center">
            <Mic className="mx-auto mb-3 size-8 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              No mock interviews yet. Start one and your results will show up here.
            </p>
            <Button asChild variant="brand" size="sm" className="mt-4">
              <Link to="/interview">
                Start a mock interview <ArrowRight className="size-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-3">
            {shown.map((iv) => (
              <InterviewRow key={iv.id} interview={iv} />
            ))}
          </div>
          {hasMore && (
            <div className="flex justify-center pt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setVisible((v) => v + INTERVIEW_PAGE_SIZE)}
              >
                Show more
              </Button>
            </div>
          )}
          {!hasMore && visible > INTERVIEW_PAGE_SIZE && (
            <div className="flex justify-center pt-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setVisible(INTERVIEW_PAGE_SIZE)}
              >
                Show less
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  )
}

function OnboardingNudge() {
  return (
    <Card className="overflow-hidden border-brand-500/40">
      <div className="grid gap-0 md:grid-cols-[1fr_auto]">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Sparkles className="size-4 text-brand-300" />
            <CardTitle className="text-base">Finish setting up your profile</CardTitle>
          </div>
          <CardDescription>
            Two minutes — and the rest of the app starts working for you.
            Recommendations, adaptive mock interviews, and per-question feedback all get sharper
            once we know your role, tech, and goals. You can drop in a resume to auto-fill.
          </CardDescription>
        </CardHeader>
        <div className="flex items-center px-6 pb-6 md:py-6">
          <Button asChild variant="brand" className="w-full sm:w-auto">
            <Link to="/onboarding">
              <UserCircle2 className="size-4" /> Set up profile
              <ArrowRight className="size-4" />
            </Link>
          </Button>
        </div>
      </div>
    </Card>
  )
}

function InterviewRow({ interview }: { interview: Interview }) {
  const score = interview.score == null ? null : Math.round(interview.score)
  const tone =
    score == null
      ? 'text-muted-foreground'
      : score >= 80
        ? 'text-emerald-300'
        : score >= 55
          ? 'text-brand-200'
          : 'text-red-300'
  const StatusIcon =
    interview.status === 'completed'
      ? CheckCircle2
      : interview.status === 'abandoned'
        ? TriangleAlert
        : PlayCircle
  const date = new Date(interview.startedAt).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
  })
  const modeLabel = interview.mode === 'adaptive' ? 'Adaptive' : 'Topic'

  return (
    <Link to={`/interview/${interview.id}`}>
      <Card className="transition-colors hover:border-brand-500/40">
        <CardHeader className="flex flex-row items-center justify-between gap-3 space-y-0 py-4">
          <div className="flex min-w-0 items-center gap-3">
            <div className="grid size-9 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">
              <StatusIcon className="size-4" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="text-sm font-medium capitalize">{interview.status.replace('_', ' ')}</span>
                <Badge variant="outline" className="text-muted-foreground">
                  {modeLabel}
                </Badge>
                <span className="text-xs text-muted-foreground">{date}</span>
              </div>
              {interview.summary && (
                <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">{interview.summary}</p>
              )}
            </div>
          </div>
          {score != null && (
            <div className={`shrink-0 font-mono text-lg font-bold ${tone}`}>{score}</div>
          )}
        </CardHeader>
      </Card>
    </Link>
  )
}
