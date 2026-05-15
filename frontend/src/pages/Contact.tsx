import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, Check, Copy, Mail, MessageSquare, Sparkles } from 'lucide-react'

import { useSEO } from '@/hooks/useSEO'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

const CONTACT_EMAIL = 'contact@10xinterview.com'

const REASONS = [
  {
    icon: MessageSquare,
    title: 'Feedback or feature request',
    body: 'Tell us what would make your prep faster. We read every message.',
  },
  {
    icon: Sparkles,
    title: 'Bug report',
    body: 'Something broken? Send a short description and we’ll look right away.',
  },
  {
    icon: Mail,
    title: 'Partnerships & press',
    body: 'Reach out about content, partnerships, or coverage of 10xInterview.',
  },
]

export function Contact() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const [copied, setCopied] = useState(false)

  useSEO({
    title: 'Contact 10xInterview — Feedback, Bugs & Partnerships',
    description:
      'Reach the 10xInterview team for feedback, feature requests, bug reports, partnerships, or press. We reply within a day or two.',
    path: '/contact',
  })

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const body = `${message}\n\n— ${name || 'Anonymous'}${email ? ` (${email})` : ''}`
    const mailto = `mailto:${CONTACT_EMAIL}?subject=${encodeURIComponent(
      subject || 'Hello from 10xInterview',
    )}&body=${encodeURIComponent(body)}`
    window.location.href = mailto
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(CONTACT_EMAIL)
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      // clipboard blocked — fall back to selecting text
    }
  }

  return (
    <div className="relative isolate">
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 grid-bg radial-fade opacity-60" aria-hidden />
        <div
          aria-hidden
          className="absolute left-1/2 top-[-10rem] -z-10 h-[40rem] w-[60rem] -translate-x-1/2 rounded-full bg-gradient-to-br from-brand-500/30 via-fuchsia-500/15 to-transparent blur-3xl"
        />

        <div className="mx-auto max-w-6xl px-4 pb-16 pt-20 sm:px-6 sm:pt-24 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <Badge variant="brand" className="mb-6 gap-1.5 px-3 py-1">
              <Mail className="size-3" />
              Get in touch
            </Badge>
            <h1 className="text-balance text-4xl font-bold tracking-tight sm:text-5xl">
              We’d love to hear from you.
            </h1>
            <p className="mx-auto mt-5 max-w-xl text-pretty text-base text-muted-foreground sm:text-lg">
              Questions, feedback, or just want to say hi? Drop us a message and we’ll get back within a day or two.
            </p>
          </div>

          <div className="mx-auto mt-14 grid max-w-5xl gap-8 lg:grid-cols-[1fr_1.1fr]">
            {/* Left: contact info */}
            <div className="space-y-6">
              <Card className="border-border/60 bg-card/60">
                <CardHeader>
                  <CardTitle className="text-base">Email us directly</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-background/40 px-3 py-2 text-sm">
                    <Mail className="size-4 shrink-0 text-brand-300" />
                    <span className="truncate font-mono text-foreground">{CONTACT_EMAIL}</span>
                    <button
                      type="button"
                      onClick={handleCopy}
                      className="ml-auto inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                      aria-label="Copy email address"
                    >
                      {copied ? (
                        <>
                          <Check className="size-3.5 text-emerald-400" /> Copied
                        </>
                      ) : (
                        <>
                          <Copy className="size-3.5" /> Copy
                        </>
                      )}
                    </button>
                  </div>
                  <Button asChild variant="outline" className="w-full">
                    <a href={`mailto:${CONTACT_EMAIL}`}>
                      Open in mail app <ArrowRight className="size-4" />
                    </a>
                  </Button>
                </CardContent>
              </Card>

              <div className="space-y-3">
                {REASONS.map(({ icon: Icon, title, body }) => (
                  <div
                    key={title}
                    className="flex gap-3 rounded-xl border border-border/60 bg-card/40 p-4"
                  >
                    <div className="inline-flex size-9 shrink-0 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                      <Icon className="size-4" />
                    </div>
                    <div>
                      <h3 className="text-sm font-semibold">{title}</h3>
                      <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{body}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Right: form */}
            <Card className="border-border/70 bg-card/70 shadow-2xl shadow-brand-900/20">
              <CardHeader>
                <CardTitle className="text-lg">Send a message</CardTitle>
                <p className="text-sm text-muted-foreground">
                  Fill this out — we’ll open your mail app with it ready to send.
                </p>
              </CardHeader>
              <CardContent>
                <form onSubmit={handleSubmit} className="space-y-4">
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="space-y-1.5">
                      <Label htmlFor="name">Your name</Label>
                      <Input
                        id="name"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        placeholder="Jane Doe"
                        autoComplete="name"
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="email">Your email</Label>
                      <Input
                        id="email"
                        type="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        placeholder="you@example.com"
                        autoComplete="email"
                      />
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="subject">Subject</Label>
                    <Input
                      id="subject"
                      value={subject}
                      onChange={(e) => setSubject(e.target.value)}
                      placeholder="What’s this about?"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="message">Message</Label>
                    <Textarea
                      id="message"
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                      placeholder="Tell us what’s on your mind…"
                      rows={6}
                      required
                    />
                  </div>

                  <Button type="submit" variant="brand" size="lg" className="w-full">
                    Send message <ArrowRight className="size-4" />
                  </Button>
                  <p className="text-center text-xs text-muted-foreground">
                    This opens your default mail app. Prefer to copy the address?{' '}
                    <button
                      type="button"
                      onClick={handleCopy}
                      className="text-brand-300 underline-offset-4 hover:underline"
                    >
                      Copy email
                    </button>
                  </p>
                </form>
              </CardContent>
            </Card>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-5xl px-4 pb-24 sm:px-6 lg:px-8">
        <div className="relative overflow-hidden rounded-2xl border border-border/60 bg-gradient-to-br from-brand-950/80 via-card to-card p-10 text-center shadow-xl">
          <div
            aria-hidden
            className="absolute left-1/2 top-0 -z-10 h-40 w-[40rem] -translate-x-1/2 rounded-full bg-brand-500/30 blur-3xl"
          />
          <h2 className="text-balance text-2xl font-bold tracking-tight sm:text-3xl">
            While you’re here — try a mock interview.
          </h2>
          <p className="mx-auto mt-3 max-w-xl text-sm text-muted-foreground">
            See what 10xInterview can do in under a minute. No card, no commitment.
          </p>
          <div className="mt-7">
            <Button asChild size="xl" variant="brand">
              <Link to="/">
                Back to home <ArrowRight className="size-4" />
              </Link>
            </Button>
          </div>
        </div>
      </section>
    </div>
  )
}
