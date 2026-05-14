import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import {
  ArrowRight,
  CheckCircle2,
  FileText,
  Loader2,
  Sparkles,
  Upload,
  X,
} from 'lucide-react'

import {
  useCategories,
  useProfile,
  useUploadResume,
  useUpsertProfile,
} from '@/hooks/queries'
import type { Seniority } from '@/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

const SENIORITIES: { value: Seniority; label: string }[] = [
  { value: 'junior', label: 'Junior' },
  { value: 'mid', label: 'Mid-level' },
  { value: 'senior', label: 'Senior' },
  { value: 'staff', label: 'Staff' },
  { value: 'principal', label: 'Principal' },
]

const schema = z.object({
  targetRole: z.string().min(1, 'Pick the role you’re targeting.'),
  yearsExperience: z
    .number({ error: 'Enter a valid number.' })
    .int()
    .min(0)
    .max(60),
  seniority: z.enum(['', 'junior', 'mid', 'senior', 'staff', 'principal']).optional(),
  currentRole: z.string().max(200).optional(),
  techStackRaw: z.string().max(500).optional(),
  goals: z.string().max(500).optional(),
})

type FormValues = z.infer<typeof schema>

export function Onboarding() {
  const navigate = useNavigate()
  const { data: profile, isLoading } = useProfile()
  const { data: categories } = useCategories()
  const upsert = useUpsertProfile()
  const upload = useUploadResume()

  const [techChips, setTechChips] = useState<string[]>([])
  const [uploadErr, setUploadErr] = useState<string | null>(null)

  const roles = (categories ?? []).filter((c) => c.kind === 'role')

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
    setValue,
    watch,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      targetRole: '',
      yearsExperience: 0,
      seniority: '',
      currentRole: '',
      techStackRaw: '',
      goals: '',
    },
  })

  // When profile data arrives (e.g., after resume upload), prefill the form.
  const profileSig = useMemo(
    () =>
      profile
        ? `${profile.targetRole}|${profile.yearsExperience}|${profile.seniority}|${profile.currentRole}|${profile.techStack.join(',')}|${profile.goals}`
        : '',
    [profile],
  )
  useEffect(() => {
    if (!profile) return
    reset({
      targetRole: profile.targetRole,
      yearsExperience: profile.yearsExperience,
      seniority: profile.seniority,
      currentRole: profile.currentRole,
      techStackRaw: '',
      goals: profile.goals,
    })
    setTechChips(profile.techStack)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileSig])

  const targetRole = watch('targetRole')
  const seniority = watch('seniority')

  const onTechAdd = (raw: string) => {
    const parts = raw
      .split(/[,\n]/)
      .map((s) => s.trim())
      .filter(Boolean)
    if (parts.length === 0) return
    setTechChips((prev) => {
      const seen = new Set(prev.map((s) => s.toLowerCase()))
      const next = [...prev]
      for (const p of parts) {
        if (!seen.has(p.toLowerCase())) {
          next.push(p)
          seen.add(p.toLowerCase())
        }
      }
      return next.slice(0, 30)
    })
    setValue('techStackRaw', '')
  }

  const removeTech = (slug: string) => {
    setTechChips((prev) => prev.filter((s) => s !== slug))
  }

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    await upsert.mutateAsync({
      targetRole: values.targetRole,
      yearsExperience: values.yearsExperience,
      seniority: (values.seniority || '') as Seniority,
      currentRole: values.currentRole || '',
      techStack: techChips,
      goals: values.goals || '',
      markOnboarded: true,
    })
    navigate('/dashboard')
  }

  const onResume = async (file: File) => {
    setUploadErr(null)
    try {
      await upload.mutateAsync(file)
    } catch (err) {
      setUploadErr((err as Error)?.message || 'Resume parsing failed.')
    }
  }

  if (isLoading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading profile…
      </div>
    )
  }

  return (
    <section className="space-y-8">
      <header className="space-y-2">
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          {profile?.onboardedAt ? 'Edit your profile' : 'Get set up'}
        </p>
        <h1 className="text-3xl font-bold tracking-tight">Tell us about you</h1>
        <p className="text-muted-foreground">
          We use this to recommend questions and tailor adaptive mock interviews. You can edit any
          time. Fastest path: drop your resume below and we’ll extract the basics.
        </p>
      </header>

      <ResumeDropzone
        currentName={profile?.resumeFilename}
        busy={upload.isPending}
        error={uploadErr}
        onFile={onResume}
      />

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">1. What role are you targeting?</CardTitle>
            <CardDescription>This drives the questions you’ll see on the dashboard.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {roles.map((r) => (
                <button
                  key={r.slug}
                  type="button"
                  onClick={() => setValue('targetRole', r.slug, { shouldValidate: true })}
                  className={cn(
                    'rounded-md border px-3 py-2 text-sm font-medium transition-colors',
                    targetRole === r.slug
                      ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                      : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                  )}
                >
                  {r.name}
                </button>
              ))}
              {roles.length === 0 && <span className="text-xs text-muted-foreground">Loading…</span>}
            </div>
            {errors.targetRole && (
              <p className="mt-2 text-xs text-red-300">{errors.targetRole.message}</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">2. Experience</CardTitle>
            <CardDescription>How long have you been writing software professionally?</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="yearsExperience">Years of experience</Label>
              <Input
                id="yearsExperience"
                type="number"
                min={0}
                max={60}
                {...register('yearsExperience', { valueAsNumber: true })}
              />
            </div>
            <div className="grid gap-1.5">
              <Label>Seniority</Label>
              <div className="flex flex-wrap gap-2">
                {SENIORITIES.map((s) => (
                  <button
                    key={s.value}
                    type="button"
                    onClick={() => setValue('seniority', s.value, { shouldDirty: true })}
                    className={cn(
                      'rounded-md border px-3 py-1.5 text-xs font-medium transition-colors',
                      seniority === s.value
                        ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                        : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                    )}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">3. Current role (optional)</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-1.5">
            <Input
              id="currentRole"
              placeholder="Backend Engineer at Acme"
              {...register('currentRole')}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">4. Tech you’ve worked with</CardTitle>
            <CardDescription>
              Add a few. We’ll prioritize questions in these areas. Press Enter or comma to add.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Input
              placeholder="Go, Postgres, Kubernetes…"
              {...register('techStackRaw')}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ',') {
                  e.preventDefault()
                  onTechAdd((e.currentTarget as HTMLInputElement).value)
                }
              }}
              onBlur={(e) => onTechAdd(e.currentTarget.value)}
            />
            {techChips.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {techChips.map((t) => (
                  <Badge
                    key={t}
                    variant="brand"
                    className="cursor-pointer gap-1"
                    onClick={() => removeTech(t)}
                    role="button"
                  >
                    {t} <X className="size-3" />
                  </Badge>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">5. What are you preparing for? (optional)</CardTitle>
            <CardDescription>A sentence or two. We pass this to the AI interviewer.</CardDescription>
          </CardHeader>
          <CardContent>
            <Textarea
              rows={3}
              placeholder="Aiming for senior backend roles. Want to be sharper on system design."
              {...register('goals')}
            />
          </CardContent>
        </Card>

        {upsert.isError && (
          <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
            {(upsert.error as Error)?.message || 'Save failed.'}
          </p>
        )}

        <div className="flex justify-end gap-3">
          <Button type="submit" variant="brand" size="xl" disabled={upsert.isPending}>
            {upsert.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin" /> Saving…
              </>
            ) : (
              <>
                <CheckCircle2 className="size-4" /> Save and continue
                <ArrowRight className="size-4" />
              </>
            )}
          </Button>
        </div>
      </form>
    </section>
  )
}

function ResumeDropzone({
  currentName,
  busy,
  error,
  onFile,
}: {
  currentName?: string
  busy: boolean
  error: string | null
  onFile: (file: File) => void
}) {
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [dragging, setDragging] = useState(false)

  const handle = (file?: File | null) => {
    if (!file) return
    onFile(file)
  }

  return (
    <Card
      className={cn(
        'overflow-hidden border-dashed transition-colors',
        dragging && 'border-brand-400/60 bg-brand-500/5',
      )}
      onDragOver={(e) => {
        e.preventDefault()
        if (!dragging) setDragging(true)
      }}
      onDragLeave={(e) => {
        // Only clear when leaving the card itself (not when entering a child).
        if (e.currentTarget === e.target) setDragging(false)
      }}
      onDrop={(e) => {
        e.preventDefault()
        setDragging(false)
        handle(e.dataTransfer.files?.[0])
      }}
    >
      <CardContent className="flex flex-col items-center gap-3 py-8 text-center">
        <div
          className={cn(
            'inline-flex size-12 items-center justify-center rounded-xl',
            'bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30',
          )}
        >
          {busy ? <Loader2 className="size-5 animate-spin" /> : <Sparkles className="size-5" />}
        </div>
        <div>
          <h3 className="text-base font-semibold">
            {busy ? 'Reading your resume…' : 'Drop your resume to auto-fill'}
          </h3>
          <p className="mt-1 text-sm text-muted-foreground">
            PDF (preferred) or plain text. We extract experience, seniority, tech stack, and goals.
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => inputRef.current?.click()}
          >
            <Upload className="size-4" /> Choose file
          </Button>
          {currentName && (
            <span className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-card/40 px-2 py-1 text-xs text-muted-foreground">
              <FileText className="size-3.5" /> {currentName}
            </span>
          )}
        </div>
        <input
          ref={inputRef}
          type="file"
          accept="application/pdf,text/plain,.pdf,.txt"
          className="hidden"
          onChange={(e) => handle(e.target.files?.[0])}
          disabled={busy}
        />
        {error && (
          <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-red-300">
            {error}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
