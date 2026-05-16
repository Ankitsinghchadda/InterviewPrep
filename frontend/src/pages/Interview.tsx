import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  ArrowRight,
  Briefcase,
  CheckCircle2,
  Clock,
  FileText,
  Layers,
  Loader2,
  Lock,
  Mic,
  MessagesSquare,
  PlayCircle,
  Sparkles,
  UserCircle2,
  Zap,
} from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import { useCategories, useProfile, useStartInterview } from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { LiveJobDescriptionDialog } from '@/components/LiveJobDescriptionDialog'
import { cn } from '@/lib/utils'

const COUNTS = [3, 5, 8]
const DURATIONS = [15, 30, 45] as const

type Mode = 'topic' | 'adaptive' | 'live'

export function Interview() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const { data: categories, isLoading: catsLoading } = useCategories()
  const { data: profile } = useProfile()
  const start = useStartInterview()
  const isPro = user?.plan === 'pro'

  const [mode, setMode] = useState<Mode>('topic')
  const [selectedSlugs, setSelectedSlugs] = useState<Set<string>>(new Set())
  const [count, setCount] = useState(5)
  const [durationMinutes, setDurationMinutes] = useState<number>(30)
  const [jobDescription, setJobDescription] = useState('')
  const [jdOpen, setJdOpen] = useState(false)

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
      const jd = jobDescription.trim()
      const iv = await start.mutateAsync({
        mode,
        categories: mode === 'topic' ? Array.from(selectedSlugs) : [],
        count: mode === 'live' ? undefined : count,
        durationMinutes: mode === 'live' ? durationMinutes : undefined,
        jobDescription: mode === 'live' && jd ? jd : undefined,
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
              {!isPro && (
                <span className="ml-1 inline-flex items-center gap-0.5 rounded bg-brand-500/20 px-1.5 py-0.5 text-[10px] font-semibold text-brand-200">
                  <Lock className="size-2.5" /> Pro
                </span>
              )}
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
          {!isPro ? (
            <LiveProGate />
          ) : (
            <LiveExplainer
              durationMinutes={durationMinutes}
              onDurationChange={setDurationMinutes}
              profile={profile}
              jobDescription={jobDescription}
              onOpenJobDescription={() => setJdOpen(true)}
              onClearJobDescription={() => setJobDescription('')}
            />
          )}
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
          disabled={
            start.isPending ||
            (mode === 'adaptive' && !onboarded) ||
            (mode === 'live' && !isPro)
          }
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

      <LiveJobDescriptionDialog
        open={jdOpen}
        onOpenChange={setJdOpen}
        value={jobDescription}
        onSave={setJobDescription}
      />
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
  jobDescription,
  onOpenJobDescription,
  onClearJobDescription,
}: {
  durationMinutes: number
  onDurationChange: (n: number) => void
  profile: ReturnType<typeof useProfile>['data']
  jobDescription: string
  onOpenJobDescription: () => void
  onClearJobDescription: () => void
}) {
  const hasJd = jobDescription.trim().length > 0

  const resumeHint = profile?.resumeFilename
    ? `Pulls from ${profile.resumeFilename}`
    : 'Uses your profile (if set)'

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-brand-500/30 bg-brand-500/5 p-4">
        <div className="flex items-start gap-3">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-brand-500/15 text-brand-300">
            <MessagesSquare className="size-4" />
          </div>
          <div className="min-w-0 flex-1 space-y-2">
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
              <h3 className="text-sm font-semibold text-foreground">Real-interview simulation</h3>
              <p className="text-xs text-muted-foreground">
                Feedback hidden until the interview ends — just like the real thing.
              </p>
            </div>
            <div className="flex flex-wrap gap-1.5">
              <FeatureChip icon={Mic} label="Voice-only" />
              <FeatureChip icon={Zap} label="Dynamic follow-ups" />
              <FeatureChip icon={FileText} label={resumeHint} />
            </div>
          </div>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card className="flex flex-col">
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <Clock className="size-4 text-brand-300" />
              <CardTitle className="text-base">How long?</CardTitle>
            </div>
            <CardDescription>
              The clock counts down once you start. You can finish your current answer when time's up.
            </CardDescription>
          </CardHeader>
          <CardContent className="mt-auto">
            <div className="grid grid-cols-3 gap-2">
              {DURATIONS.map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => onDurationChange(n)}
                  className={cn(
                    'rounded-md border px-3 py-2 text-sm font-medium transition-colors',
                    durationMinutes === n
                      ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                      : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                  )}
                >
                  {n} min
                </button>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card className="flex flex-col">
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <FileText className="size-4 text-brand-300" />
              <CardTitle className="text-base">
                Target role{' '}
                <span className="text-xs font-normal text-muted-foreground">(optional)</span>
              </CardTitle>
            </div>
            <CardDescription>
              Paste a job description to tailor questions to a specific listing.
            </CardDescription>
          </CardHeader>
          <CardContent className="mt-auto">
            <div className="flex flex-wrap items-center justify-between gap-2">
              {hasJd ? (
                <div className="flex items-center gap-1.5 text-sm">
                  <CheckCircle2 className="size-4 text-emerald-400" />
                  <span className="text-foreground">JD added</span>
                  <span className="text-xs text-muted-foreground">
                    · {jobDescription.length.toLocaleString()} chars
                  </span>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No JD added.</p>
              )}
              <div className="flex gap-1.5">
                <Button type="button" variant="outline" size="sm" onClick={onOpenJobDescription}>
                  {hasJd ? 'Edit' : 'Add JD'}
                </Button>
                {hasJd && (
                  <Button type="button" variant="ghost" size="sm" onClick={onClearJobDescription}>
                    Clear
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function FeatureChip({
  icon: Icon,
  label,
}: {
  icon: React.ComponentType<{ className?: string }>
  label: string
}) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-border/60 bg-card/60 px-2 py-0.5 text-[11px] text-muted-foreground">
      <Icon className="size-3 text-brand-300" />
      {label}
    </span>
  )
}

// LiveProGate replaces the LiveExplainer for free users. Click the Start
// button below — backend returns 403 plan_required, the api interceptor
// opens the PaywallModal. We also expose an inline upgrade CTA here so
// users don't have to hit a wall before discovering it's paid.
function LiveProGate() {
  return (
    <Card className="border-brand-500/40">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Lock className="size-4 text-brand-300" />
          <CardTitle className="text-base">Live interview is a Pro feature</CardTitle>
        </div>
        <CardDescription>
          Live mode runs on the premium Gemini model — one question at a time, dynamic follow-ups
          based on your last answer, voice-only, just like a real interview. Upgrade to unlock it
          alongside unlimited AI reviews and mock interviews.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Button asChild variant="brand">
          <Link to="/pricing">
            See Pro plans <ArrowRight className="size-4" />
          </Link>
        </Button>
      </CardContent>
    </Card>
  )
}
