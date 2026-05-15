import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  ArrowRight,
  Briefcase,
  Clock,
  Layers,
  Loader2,
  MessagesSquare,
  PlayCircle,
  Sparkles,
  UserCircle2,
} from 'lucide-react'

import { useCategories, useProfile, useStartInterview } from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

const COUNTS = [3, 5, 8]
const DURATIONS = [15, 30, 45] as const

type Mode = 'topic' | 'adaptive' | 'live'

export function Interview() {
  const navigate = useNavigate()
  const { data: categories, isLoading: catsLoading } = useCategories()
  const { data: profile } = useProfile()
  const start = useStartInterview()

  const [mode, setMode] = useState<Mode>('topic')
  const [selectedSlugs, setSelectedSlugs] = useState<Set<string>>(new Set())
  const [count, setCount] = useState(5)
  const [durationMinutes, setDurationMinutes] = useState<number>(30)

  const roles = useMemo(() => (categories ?? []).filter((c) => c.kind === 'role'), [categories])
  const topics = useMemo(() => (categories ?? []).filter((c) => c.kind === 'topic'), [categories])

  const onboarded = Boolean(profile?.onboardedAt)

  const toggle = (slug: string) => {
    setSelectedSlugs((prev) => {
      const next = new Set(prev)
      if (next.has(slug)) next.delete(slug)
      else next.add(slug)
      return next
    })
  }

  const onStart = async () => {
    try {
      const iv = await start.mutateAsync({
        mode,
        categories: mode === 'topic' ? Array.from(selectedSlugs) : [],
        count: mode === 'live' ? undefined : count,
        durationMinutes: mode === 'live' ? durationMinutes : undefined,
      })
      navigate(`/interview/${iv.id}`)
    } catch (err) {
      console.error('start interview failed', err)
    }
  }

  return (
    <section className="space-y-6 sm:space-y-8">
      <header className="space-y-2">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Mock Interview</h1>
        <p className="text-sm text-muted-foreground sm:text-base">
          Pick a mode and how many questions you want, then start the recorded session.
        </p>
      </header>

      <Tabs value={mode} onValueChange={(v) => setMode(v as Mode)}>
        <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:overflow-visible sm:px-0">
          <TabsList className="h-auto w-max min-w-full justify-start sm:w-auto sm:min-w-0">
            <TabsTrigger value="topic" className="whitespace-nowrap">
              <Layers className="size-4" /> Topic-based
            </TabsTrigger>
            <TabsTrigger value="adaptive" className="whitespace-nowrap">
              <Sparkles className="size-4" />
              <span className="sm:hidden">Adaptive</span>
              <span className="hidden sm:inline">Adaptive (uses your profile)</span>
            </TabsTrigger>
            <TabsTrigger value="live" className="whitespace-nowrap">
              <MessagesSquare className="size-4" /> Live Interview
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="topic" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Choose categories</CardTitle>
              <CardDescription>
                Leave both empty for a random mix across the curated library.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <CategorySection
                title="Roles"
                icon={Briefcase}
                items={roles.map((c) => ({ slug: c.slug, name: c.name }))}
                loading={catsLoading}
                selected={selectedSlugs}
                onToggle={toggle}
                tone="role"
              />
              <CategorySection
                title="Topics"
                icon={Layers}
                items={topics.map((c) => ({ slug: c.slug, name: c.name }))}
                loading={catsLoading}
                selected={selectedSlugs}
                onToggle={toggle}
                tone="topic"
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="adaptive" className="space-y-6">
          <AdaptiveExplainer profile={profile} onboarded={onboarded} />
        </TabsContent>

        <TabsContent value="live" className="space-y-6">
          <LiveExplainer
            durationMinutes={durationMinutes}
            onDurationChange={setDurationMinutes}
            profile={profile}
          />
        </TabsContent>
      </Tabs>

      {mode !== 'live' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">How many questions?</CardTitle>
            <CardDescription>3 is a quick warm-up, 8 is a full mock.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {COUNTS.map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => setCount(n)}
                  className={cn(
                    'rounded-md border px-4 py-2 text-sm font-medium transition-colors',
                    count === n
                      ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                      : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                  )}
                >
                  {n} questions
                </button>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {start.isError && (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
          {(start.error as Error)?.message ||
            'Could not start the interview. Try a different mix.'}
        </p>
      )}

      <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
        <Button
          size="xl"
          variant="brand"
          onClick={onStart}
          disabled={start.isPending || (mode === 'adaptive' && !onboarded)}
          className="w-full sm:w-auto"
        >
          {start.isPending ? (
            <>
              <Loader2 className="size-4 animate-spin" />{' '}
              {mode === 'adaptive'
                ? 'Designing your interview…'
                : mode === 'live'
                  ? 'Warming up the interviewer…'
                  : 'Preparing…'}
            </>
          ) : (
            <>
              <PlayCircle className="size-4" />{' '}
              {mode === 'live' ? 'Start live interview' : 'Start mock interview'}
              <ArrowRight className="size-4" />
            </>
          )}
        </Button>
      </div>
    </section>
  )
}

function AdaptiveExplainer({
  profile,
  onboarded,
}: {
  profile: ReturnType<typeof useProfile>['data']
  onboarded: boolean
}) {
  if (!onboarded) {
    return (
      <Card className="border-brand-500/40">
        <CardHeader>
          <div className="flex items-center gap-2">
            <UserCircle2 className="size-4 text-brand-300" />
            <CardTitle className="text-base">Profile required</CardTitle>
          </div>
          <CardDescription>
            Adaptive interviews are built from your role, experience, tech stack, and (optionally)
            resume. Finish setup once and every adaptive interview after that is tailored.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant="brand">
            <Link to="/onboarding">
              Finish profile <ArrowRight className="size-4" />
            </Link>
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Sparkles className="size-4 text-brand-300" />
          <CardTitle className="text-base">Tailored to your profile</CardTitle>
        </div>
        <CardDescription>
          The interviewer agent will mimic a real interview: an intro question, a behavioral pull
          from your background, a deep dive into your primary tech, and a system-design question
          scoped to your seniority.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <ul className="space-y-2 text-sm">
          <li className="flex items-center gap-2">
            <span className="text-xs uppercase tracking-wider text-muted-foreground">Role</span>
            <span className="text-foreground">{profile?.targetRole || '—'}</span>
          </li>
          <li className="flex items-center gap-2">
            <span className="text-xs uppercase tracking-wider text-muted-foreground">Seniority</span>
            <span className="text-foreground">
              {profile?.seniority || 'unspecified'}
              {profile?.yearsExperience ? ` (${profile.yearsExperience}y)` : ''}
            </span>
          </li>
          <li className="flex flex-wrap items-center gap-2">
            <span className="text-xs uppercase tracking-wider text-muted-foreground">Tech</span>
            <span className="text-foreground">
              {profile?.techStack.length ? profile.techStack.slice(0, 6).join(', ') : '—'}
            </span>
          </li>
          {profile?.resumeFilename && (
            <li className="flex items-center gap-2">
              <span className="text-xs uppercase tracking-wider text-muted-foreground">Resume</span>
              <span className="text-foreground">{profile.resumeFilename}</span>
            </li>
          )}
        </ul>
        <p className="text-xs text-muted-foreground">
          Need to change anything?{' '}
          <Link to="/onboarding" className="text-brand-300 hover:underline">
            Edit profile
          </Link>
          .
        </p>
      </CardContent>
    </Card>
  )
}

function CategorySection({
  title,
  icon: Icon,
  items,
  loading,
  selected,
  onToggle,
  tone,
}: {
  title: string
  icon: React.ComponentType<{ className?: string }>
  items: { slug: string; name: string }[]
  loading: boolean
  selected: Set<string>
  onToggle: (slug: string) => void
  tone: 'role' | 'topic'
}) {
  return (
    <div>
      <div className="mb-2 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
        <Icon className="size-3.5" />
        {title}
      </div>
      <div className="flex flex-wrap gap-2">
        {loading && <span className="text-xs text-muted-foreground">Loading…</span>}
        {items.map((c) => {
          const active = selected.has(c.slug)
          return (
            <button
              key={c.slug}
              type="button"
              onClick={() => onToggle(c.slug)}
              className={cn(
                'rounded-full border px-3 py-1 text-xs font-medium transition-colors',
                active
                  ? tone === 'role'
                    ? 'border-brand-400/60 bg-brand-500/20 text-brand-100'
                    : 'border-emerald-500/60 bg-emerald-500/15 text-emerald-200'
                  : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
              )}
            >
              {c.name}
            </button>
          )
        })}
      </div>
    </div>
  )
}

function LiveExplainer({
  durationMinutes,
  onDurationChange,
  profile,
}: {
  durationMinutes: number
  onDurationChange: (n: number) => void
  profile: ReturnType<typeof useProfile>['data']
}) {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <MessagesSquare className="size-4 text-brand-300" />
            <CardTitle className="text-base">Real-interview simulation</CardTitle>
          </div>
          <CardDescription>
            One question at a time, voice-only, with an AI interviewer that asks dynamic
            follow-ups based on your previous answer
            {profile?.resumeFilename ? ' and your uploaded resume' : ' and your profile (if set)'}.
            Feedback is hidden until the interview ends — just like the real thing.
          </CardDescription>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Clock className="size-4 text-brand-300" />
            <CardTitle className="text-base">How long?</CardTitle>
          </div>
          <CardDescription>
            The clock counts down once you start. When time's up, you can finish your current
            answer and the interview will wrap.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {DURATIONS.map((n) => (
              <button
                key={n}
                type="button"
                onClick={() => onDurationChange(n)}
                className={cn(
                  'rounded-md border px-4 py-2 text-sm font-medium transition-colors',
                  durationMinutes === n
                    ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                    : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                )}
              >
                {n} minutes
              </button>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
