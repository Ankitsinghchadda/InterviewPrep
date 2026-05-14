import { Link } from 'react-router-dom'
import {
  ArrowRight,
  CheckCircle2,
  ListChecks,
  Mic,
  PlayCircle,
  Sparkles,
  TriangleAlert,
  UserCircle2,
} from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import {
  useMyInterviews,
  useProfile,
  useRecommendedQuestions,
} from '@/hooks/queries'
import type { Interview, Question } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

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
  const { data: recommended, isLoading: recsLoading } = useRecommendedQuestions()
  const { data: interviews } = useMyInterviews()

  const firstName = user?.name?.split(' ')[0] || 'there'
  const onboarded = Boolean(profile?.onboardedAt)
  const recentInterviews = (interviews ?? []).slice(0, 3)

  return (
    <section className="space-y-10">
      <header>
        <p className="text-sm text-muted-foreground">Welcome back</p>
        <h1 className="mt-1 text-3xl font-bold tracking-tight sm:text-4xl">
          Hey {firstName} — {onboarded ? 'what do you want to drill today?' : 'let’s tailor this to you.'}
        </h1>
      </header>

      {!onboarded && <OnboardingNudge />}

      <div className="grid gap-4 md:grid-cols-3">
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

      {onboarded && (
        <section className="space-y-3">
          <div className="flex items-baseline justify-between">
            <div>
              <h2 className="text-lg font-semibold tracking-tight">Recommended for you</h2>
              <p className="text-sm text-muted-foreground">
                Curated based on{' '}
                <span className="text-foreground">{profile?.targetRole || 'your role'}</span>
                {profile?.techStack.length ? (
                  <>
                    {' '}
                    + {profile.techStack.slice(0, 3).join(', ')}
                  </>
                ) : null}
                .
              </p>
            </div>
            <Button asChild variant="ghost" size="sm">
              <Link to="/questions">View all</Link>
            </Button>
          </div>
          {recsLoading ? (
            <RecsSkeleton />
          ) : (recommended ?? []).length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-sm text-muted-foreground">
                Nothing matched your profile yet — try broadening your tech stack in{' '}
                <Link to="/onboarding" className="text-brand-300 hover:underline">
                  your profile
                </Link>
                .
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-3 md:grid-cols-2">
              {recommended!.map((q) => (
                <RecQuestionCard key={q.id} question={q} />
              ))}
            </div>
          )}
        </section>
      )}

      <section className="space-y-3">
        <div className="flex items-baseline justify-between">
          <h2 className="text-lg font-semibold tracking-tight">Recent interviews</h2>
          {recentInterviews.length > 0 && (
            <Button asChild variant="ghost" size="sm">
              <Link to="/interview">New interview</Link>
            </Button>
          )}
        </div>
        {recentInterviews.length === 0 ? (
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
          <div className="space-y-2">
            {recentInterviews.map((iv) => (
              <InterviewRow key={iv.id} interview={iv} />
            ))}
          </div>
        )}
      </section>
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
          <Button asChild variant="brand">
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

function RecsSkeleton() {
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="h-24 animate-pulse rounded-xl border border-border/40 bg-card/40" />
      ))}
    </div>
  )
}

function RecQuestionCard({ question }: { question: Question }) {
  return (
    <Link to={`/questions/${question.id}`}>
      <Card className="h-full transition-colors hover:border-brand-500/40">
        <CardHeader>
          <div className="mb-2 flex flex-wrap items-center gap-1.5">
            <Badge variant="brand">{question.difficulty}</Badge>
            {question.categories.slice(0, 3).map((c) => (
              <Badge key={c} variant="outline" className="text-muted-foreground">
                {c}
              </Badge>
            ))}
          </div>
          <CardTitle className="text-base leading-snug">{question.title}</CardTitle>
        </CardHeader>
      </Card>
    </Link>
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
          <div className="flex items-center gap-3">
            <div className="grid size-9 place-items-center rounded-md bg-muted text-muted-foreground">
              <StatusIcon className="size-4" />
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-2">
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
            <div className={`font-mono text-lg font-bold ${tone}`}>{score}</div>
          )}
        </CardHeader>
      </Card>
    </Link>
  )
}
