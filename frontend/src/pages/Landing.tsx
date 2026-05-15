import { Link } from 'react-router-dom'
import {
  ArrowRight,
  BarChart3,
  Bot,
  Check,
  ListChecks,
  Mic,
  PlayCircle,
  Sparkles,
  Star,
  Timer,
  Waves,
} from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import { googleLoginURL } from '@/services/auth'
import { useSEO } from '@/hooks/useSEO'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const LANDING_JSONLD = {
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  mainEntity: [
    {
      '@type': 'Question',
      name: 'What is 10xInterview?',
      acceptedAnswer: {
        '@type': 'Answer',
        text: '10xInterview is an AI-powered mock interview platform. Practice real interview questions out loud, record your answers, and get honest feedback scored 0–100 from a Vertex AI agent.',
      },
    },
    {
      '@type': 'Question',
      name: 'Is 10xInterview free?',
      acceptedAnswer: {
        '@type': 'Answer',
        text: 'Yes — free to try with no credit card required. Sign in with Google and start your first mock interview in under a minute.',
      },
    },
    {
      '@type': 'Question',
      name: 'Which interview topics are covered?',
      acceptedAnswer: {
        '@type': 'Answer',
        text: 'Frontend, Backend, System Design, Docker, Kubernetes, CI/CD, Databases, and Behavioral interviews — with hundreds of curated questions.',
      },
    },
    {
      '@type': 'Question',
      name: 'How does AI feedback work?',
      acceptedAnswer: {
        '@type': 'Answer',
        text: 'You record your answer in the browser. We transcribe it and a Google Vertex AI agent grades it 0–100, lists specific strengths, identifies gaps, and suggests improvements.',
      },
    },
  ],
}

const FEATURES = [
  {
    icon: ListChecks,
    title: 'Browse curated questions',
    body: 'Hundreds of real interview questions organized by role and topic — Frontend, Backend, System Design, Docker, Kubernetes, CI/CD, and more.',
  },
  {
    icon: Mic,
    title: 'Record your answer',
    body: 'Speak your answer right in the browser. We transcribe and analyze it like a real interviewer would.',
  },
  {
    icon: Bot,
    title: 'Honest AI feedback',
    body: 'A Vertex AI agent grades your answer 0–100, calls out specific strengths, and shows exactly what to fix.',
  },
  {
    icon: PlayCircle,
    title: 'Take a mock interview',
    body: 'Pick a role and topics, get a randomized question set, and receive a full result report at the end.',
  },
  {
    icon: BarChart3,
    title: 'Track your progress',
    body: 'Every recording and review is saved. Watch your score climb week over week.',
  },
  {
    icon: Sparkles,
    title: 'Add your own questions',
    body: 'Save questions you bombed in real interviews. Build a personal library and keep coming back to them.',
  },
]

const HOW_IT_WORKS = [
  { step: '01', title: 'Pick a topic or role', body: 'Browse seeded categories or filter by what you’re weakest at.' },
  { step: '02', title: 'Record your answer', body: 'Hit record. Speak naturally — answer like you’re in the room.' },
  { step: '03', title: 'Get a real review', body: 'Score, strengths, gaps, and a rewrite suggestion — in seconds.' },
  { step: '04', title: 'Run a full mock', body: 'When you’re ready, take a timed mock interview and get a result.' },
]

const TOPIC_PILLS = [
  'Frontend',
  'Backend',
  'System Design',
  'Docker',
  'Kubernetes',
  'CI/CD',
  'Databases',
  'Behavioral',
]

export function Landing() {
  const { status, user } = useAuth()
  const authed = status === 'authenticated'

  useSEO({
    title: '10xInterview — AI Mock Interviews & Instant Feedback',
    description:
      'Practice real interview questions out loud, record your answers, and get honest AI feedback scored 0–100. Curated Frontend, Backend, System Design, Docker, Kubernetes and behavioral questions. Free to try.',
    path: '/',
    jsonLd: LANDING_JSONLD,
  })

  return (
    <div className="relative isolate">
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 grid-bg radial-fade opacity-60" aria-hidden />
        <div
          aria-hidden
          className="absolute left-1/2 top-[-10rem] -z-10 h-[40rem] w-[60rem] -translate-x-1/2 rounded-full bg-gradient-to-br from-brand-500/30 via-fuchsia-500/15 to-transparent blur-3xl"
        />
        <div className="mx-auto max-w-6xl px-4 pb-16 pt-14 text-center sm:px-6 sm:pb-24 sm:pt-28 lg:px-8">
          <Badge variant="brand" className="mb-6 gap-1.5 px-3 py-1">
            <Sparkles className="size-3" />
            Powered by Google Vertex AI
          </Badge>
          <h1 className="mx-auto max-w-4xl text-balance text-3xl font-bold tracking-tight sm:text-5xl md:text-6xl">
            Land your next role.{' '}
            <span className="bg-gradient-to-r from-brand-300 via-brand-400 to-fuchsia-400 bg-clip-text text-transparent">
              One mock interview at a time.
            </span>
          </h1>
          <p className="mx-auto mt-5 max-w-2xl text-pretty text-sm text-muted-foreground sm:mt-6 sm:text-lg">
            Practice real interview questions, record your answers out loud, and get honest, specific feedback
            from an AI agent that knows what good looks like.
          </p>

          <div className="mt-8 flex flex-col items-stretch justify-center gap-3 sm:mt-10 sm:flex-row sm:items-center">
            {authed ? (
              <>
                <Button asChild size="xl" variant="brand" className="w-full sm:w-auto">
                  <Link to="/dashboard">
                    Continue, {user?.name?.split(' ')[0] || 'welcome back'}
                    <ArrowRight className="size-4" />
                  </Link>
                </Button>
                <Button asChild size="xl" variant="outline" className="w-full sm:w-auto">
                  <Link to="/interview">Start a mock interview</Link>
                </Button>
              </>
            ) : (
              <>
                <Button asChild size="xl" variant="brand" className="w-full sm:w-auto">
                  <a href={googleLoginURL('/dashboard')}>
                    Sign in with Google
                    <ArrowRight className="size-4" />
                  </a>
                </Button>
                <Button asChild size="xl" variant="outline" className="w-full sm:w-auto">
                  <a href="#how-it-works">See how it works</a>
                </Button>
              </>
            )}
          </div>

          <p className="mt-5 text-xs text-muted-foreground">
            Free to try. No credit card. Your recordings stay private.
          </p>

          {/* Topic pills */}
          <div className="mx-auto mt-12 flex max-w-3xl flex-wrap items-center justify-center gap-2">
            {TOPIC_PILLS.map((t) => (
              <Badge key={t} variant="outline" className="border-border/70 bg-card/30 text-muted-foreground">
                {t}
              </Badge>
            ))}
          </div>
        </div>

        {/* Floating preview card */}
        <div className="mx-auto -mt-6 max-w-4xl px-4 pb-16 sm:px-6 sm:pb-24 lg:px-8">
          <div className="relative">
            <div
              aria-hidden
              className="absolute inset-x-10 -bottom-6 h-12 rounded-full bg-brand-500/30 blur-2xl"
            />
            <Card className="relative overflow-hidden border-border/70 bg-card/70 shadow-2xl shadow-brand-900/20">
              <div className="grid gap-0 md:grid-cols-[1.05fr_1fr]">
                {/* Question side */}
                <div className="border-b border-border/60 p-4 sm:p-6 md:border-b-0 md:border-r">
                  <div className="mb-3 flex flex-wrap items-center gap-2">
                    <Badge variant="brand">Backend</Badge>
                    <Badge variant="outline" className="text-muted-foreground">Medium</Badge>
                    <Badge variant="outline" className="text-muted-foreground">Docker</Badge>
                  </div>
                  <h3 className="text-base font-semibold leading-snug sm:text-lg">
                    Explain the difference between a Docker image and a container, and when you would use multi-stage builds.
                  </h3>
                  <div className="mt-5 flex items-center gap-3 sm:mt-6">
                    <button
                      type="button"
                      className="grid size-11 shrink-0 place-items-center rounded-full bg-gradient-to-br from-brand-400 to-brand-700 text-white shadow-lg shadow-brand-700/30 transition hover:scale-105 sm:size-12"
                      aria-label="Record"
                    >
                      <Mic className="size-5" />
                    </button>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <Timer className="size-3.5" /> 00:42
                      </div>
                      <div className="mt-2 flex h-8 items-end gap-1 overflow-hidden">
                        {[8, 14, 22, 16, 28, 20, 32, 18, 24, 30, 12, 22, 18, 28, 14, 20].map((h, i) => (
                          <span
                            key={i}
                            style={{ height: `${h}px` }}
                            className="w-1 rounded-full bg-gradient-to-t from-brand-600 to-brand-300"
                          />
                        ))}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Feedback side */}
                <div className="bg-gradient-to-br from-card/60 to-card p-4 sm:p-6">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 text-sm font-medium">
                      <Bot className="size-4 text-brand-300" /> AI review
                    </div>
                    <div className="flex items-center gap-1 text-sm">
                      <span className="text-2xl font-bold text-brand-300">82</span>
                      <span className="text-muted-foreground">/100</span>
                    </div>
                  </div>
                  <div className="mt-4 space-y-3 text-sm">
                    <div>
                      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-emerald-300/90">
                        Strengths
                      </div>
                      <ul className="space-y-1 text-muted-foreground">
                        <li className="flex gap-2"><Check className="mt-0.5 size-3.5 shrink-0 text-emerald-400" /> Clearly distinguished image vs running container.</li>
                        <li className="flex gap-2"><Check className="mt-0.5 size-3.5 shrink-0 text-emerald-400" /> Mentioned layer caching benefits.</li>
                      </ul>
                    </div>
                    <div>
                      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-amber-300/90">
                        Improvements
                      </div>
                      <ul className="space-y-1 text-muted-foreground">
                        <li className="flex gap-2"><Waves className="mt-0.5 size-3.5 shrink-0 text-amber-400" /> Give a concrete multi-stage example (build stage → minimal runtime).</li>
                        <li className="flex gap-2"><Waves className="mt-0.5 size-3.5 shrink-0 text-amber-400" /> Mention the security win of dropping build tools from the final image.</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </Card>
          </div>
        </div>
      </section>

      {/* Feature grid */}
      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6 sm:py-20 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-balance text-2xl font-bold tracking-tight sm:text-4xl">
            Everything you need to prep, in one place.
          </h2>
          <p className="mt-3 text-sm text-muted-foreground sm:mt-4 sm:text-base">
            Stop reading answers. Start practicing them out loud and getting feedback that actually helps.
          </p>
        </div>

        <div className="mt-10 grid gap-4 sm:mt-14 md:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, body }) => (
            <Card
              key={title}
              className="group relative overflow-hidden border-border/60 transition-colors hover:border-brand-500/40"
            >
              <div
                aria-hidden
                className="absolute -right-12 -top-12 size-32 rounded-full bg-brand-500/0 blur-2xl transition-colors group-hover:bg-brand-500/20"
              />
              <CardHeader>
                <div className="mb-3 inline-flex size-10 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                  <Icon className="size-5" />
                </div>
                <CardTitle className="text-lg">{title}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm leading-relaxed text-muted-foreground">{body}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section id="how-it-works" className="relative">
        <div
          aria-hidden
          className="absolute inset-0 -z-10 bg-gradient-to-b from-transparent via-brand-950/40 to-transparent"
        />
        <div className="mx-auto max-w-6xl px-4 py-14 sm:px-6 sm:py-20 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <Badge variant="brand" className="mb-4">How it works</Badge>
            <h2 className="text-balance text-2xl font-bold tracking-tight sm:text-4xl">
              From cold start to confident answer in four steps.
            </h2>
          </div>

          <div className="mt-10 grid gap-4 sm:mt-14 sm:gap-6 md:grid-cols-2 lg:grid-cols-4">
            {HOW_IT_WORKS.map((s, i) => (
              <div key={s.step} className="relative">
                <div className="rounded-xl border border-border/60 bg-card/50 p-5">
                  <div className="text-xs font-mono text-brand-300">{s.step}</div>
                  <h3 className="mt-2 font-semibold">{s.title}</h3>
                  <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{s.body}</p>
                </div>
                {i < HOW_IT_WORKS.length - 1 && (
                  <ArrowRight
                    aria-hidden
                    className="absolute -right-3 top-1/2 hidden size-5 -translate-y-1/2 text-border lg:block"
                  />
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Social proof / trust strip */}
      <section className="border-y border-border/60 bg-card/30">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-6 px-4 py-10 sm:px-6 md:flex-row lg:px-8">
          <div className="flex items-center gap-1 text-sm text-muted-foreground">
            <Star className="size-4 fill-amber-400 text-amber-400" />
            <Star className="size-4 fill-amber-400 text-amber-400" />
            <Star className="size-4 fill-amber-400 text-amber-400" />
            <Star className="size-4 fill-amber-400 text-amber-400" />
            <Star className="size-4 fill-amber-400 text-amber-400" />
            <span className="ml-2">Used by engineers prepping for FAANG, fintech, and startups.</span>
          </div>
          <div className="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-xs uppercase tracking-widest text-muted-foreground">
            <span>React</span>
            <span>·</span>
            <span>Go</span>
            <span>·</span>
            <span>Postgres</span>
            <span>·</span>
            <span>Vertex AI</span>
            <span>·</span>
            <span>Docker</span>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="mx-auto max-w-5xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
        <div className="relative overflow-hidden rounded-2xl border border-border/60 bg-gradient-to-br from-brand-950/80 via-card to-card p-6 text-center shadow-xl sm:p-10">
          <div
            aria-hidden
            className="absolute left-1/2 top-0 -z-10 h-40 w-[40rem] -translate-x-1/2 rounded-full bg-brand-500/30 blur-3xl"
          />
          <h2 className="text-balance text-2xl font-bold tracking-tight sm:text-4xl">
            Ready to ace your next interview?
          </h2>
          <p className="mx-auto mt-3 max-w-xl text-sm text-muted-foreground sm:text-base">
            Sign in with Google and start your first mock interview in under a minute.
          </p>
          <div className="mt-6 sm:mt-8">
            {authed ? (
              <Button asChild size="xl" variant="brand" className="w-full sm:w-auto">
                <Link to="/interview">
                  Start a mock interview <ArrowRight className="size-4" />
                </Link>
              </Button>
            ) : (
              <Button asChild size="xl" variant="brand" className="w-full sm:w-auto">
                <a href={googleLoginURL('/dashboard')}>
                  Get started — it’s free <ArrowRight className="size-4" />
                </a>
              </Button>
            )}
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-border/60">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 px-4 py-8 text-xs text-muted-foreground sm:px-6 md:flex-row lg:px-8">
          <div className="flex items-center gap-2">
            <span className="grid size-5 place-items-center rounded bg-gradient-to-br from-brand-400 to-brand-700 text-white">
              <Sparkles className="size-3" />
            </span>
            <span>© {new Date().getFullYear()} 10xInterview</span>
          </div>
          <div className="flex items-center gap-4">
            <a href="#how-it-works" className="hover:text-foreground">How it works</a>
            <Link to="/contact" className="hover:text-foreground">Contact</Link>
            <Link to="/login" className="hover:text-foreground">Sign in</Link>
          </div>
        </div>
      </footer>
    </div>
  )
}
