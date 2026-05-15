import { useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { AlertTriangle, Loader2, Plus, Sparkles } from 'lucide-react'
import { Link } from 'react-router-dom'

import {
  useCategories,
  useCreateQuestion,
  useGenerateAnswerDraft,
  useSimilarQuestions,
} from '@/hooks/queries'
import { SimilarQuestionConflict } from '@/services/questions'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import type { Difficulty, SimilarQuestion } from '@/types'

const DIFFICULTIES: Difficulty[] = ['easy', 'medium', 'hard']

// The answer field is optional in the form schema — when blank the server
// auto-generates a reference answer. When the user does fill it in we still
// enforce the same minimum length as before.
const schema = z.object({
  title: z
    .string()
    .trim()
    .min(8, 'A clear question is at least 8 characters.')
    .max(280, 'Keep the title under 280 characters.'),
  body: z.string().trim().max(2000, 'Keep context under 2000 characters.').optional(),
  answer: z
    .string()
    .trim()
    .max(8000, 'Keep the answer under 8000 characters.')
    .refine((v) => v === '' || v.length >= 20, {
      message: 'A useful reference answer is at least 20 characters (or leave blank to auto-generate).',
    }),
  difficulty: z.enum(['easy', 'medium', 'hard']),
})

type FormValues = z.infer<typeof schema>

export function NewQuestionDialog() {
  const [open, setOpen] = useState(false)
  const [selectedSlugs, setSelectedSlugs] = useState<Set<string>>(new Set())
  const [submitErr, setSubmitErr] = useState<string | null>(null)
  // Populated when the server returns a 409 — drives the confirm-or-cancel UI.
  const [conflict, setConflict] = useState<SimilarQuestion[] | null>(null)

  const { data: categories } = useCategories()
  const create = useCreateQuestion()
  const generateAnswer = useGenerateAnswerDraft()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
    watch,
    setValue,
    getValues,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { title: '', body: '', answer: '', difficulty: 'medium' },
  })
  const title = watch('title')
  const body = watch('body')
  const difficulty = watch('difficulty')

  // Live dedup panel. The hook handles debouncing + the < 8 char short-circuit.
  const similar = useSimilarQuestions({ title, body })
  const matches = (similar.data ?? []).filter((m) => m.title.trim() !== title.trim())

  const toggleSlug = (slug: string) => {
    setSelectedSlugs((prev) => {
      const next = new Set(prev)
      if (next.has(slug)) next.delete(slug)
      else next.add(slug)
      return next
    })
  }

  const onClose = (next: boolean) => {
    setOpen(next)
    if (!next) {
      reset()
      setSelectedSlugs(new Set())
      setSubmitErr(null)
      setConflict(null)
    }
  }

  const submit = async (values: FormValues, force: boolean) => {
    setSubmitErr(null)
    setConflict(null)
    try {
      await create.mutateAsync({
        title: values.title,
        body: values.body || '',
        answer: values.answer || undefined,
        difficulty: values.difficulty,
        categories: Array.from(selectedSlugs),
        force,
      })
      onClose(false)
    } catch (err) {
      if (err instanceof SimilarQuestionConflict) {
        setConflict(err.matches)
      } else {
        setSubmitErr((err as Error)?.message || 'Could not save the question.')
      }
    }
  }

  const onSubmit: SubmitHandler<FormValues> = (values) => submit(values, false)

  const onGenerate = async () => {
    setSubmitErr(null)
    const v = getValues()
    if (!v.title || v.title.trim().length < 8) {
      setSubmitErr('Write the question title first (at least 8 characters), then we can draft an answer.')
      return
    }
    try {
      const draft = await generateAnswer.mutateAsync({
        title: v.title,
        body: v.body || '',
        difficulty: v.difficulty,
        categories: Array.from(selectedSlugs),
      })
      setValue('answer', draft, { shouldDirty: true, shouldValidate: true })
    } catch (err) {
      setSubmitErr((err as Error)?.message || 'Could not draft an answer.')
    }
  }

  const roles = (categories ?? []).filter((c) => c.kind === 'role')
  const topics = (categories ?? []).filter((c) => c.kind === 'topic')

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogTrigger asChild>
        <Button variant="brand">
          <Plus className="size-4" /> New question
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add a personal question</DialogTitle>
          <DialogDescription>
            Saved to your library. Leave the answer blank and we’ll draft one for you.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
          <Field
            id="title"
            label="Question"
            error={errors.title?.message}
            hint="What you’d be asked in an interview. One sentence."
          >
            <Input
              id="title"
              placeholder="Explain the difference between processes and threads."
              aria-invalid={Boolean(errors.title)}
              {...register('title')}
            />
          </Field>

          {/* Live "looks similar to" panel — shows up as soon as the user has
              typed at least 8 chars and at least one match clears the warn
              threshold on the server (default 0.78 cosine sim). */}
          {matches.length > 0 && (
            <SimilarPanel matches={matches} onPick={() => onClose(false)} />
          )}

          <Field
            id="body"
            label="Extra context (optional)"
            error={errors.body?.message}
            hint="Hints the candidate should hear before answering."
          >
            <Textarea
              id="body"
              rows={3}
              placeholder="Mention scheduling, memory isolation, and IPC if helpful."
              aria-invalid={Boolean(errors.body)}
              {...register('body')}
            />
          </Field>

          <div className="grid gap-1.5">
            <div className="flex items-center justify-between">
              <Label htmlFor="answer">Reference answer</Label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={onGenerate}
                disabled={generateAnswer.isPending}
              >
                {generateAnswer.isPending ? (
                  <>
                    <Loader2 className="size-3.5 animate-spin" /> Drafting…
                  </>
                ) : (
                  <>
                    <Sparkles className="size-3.5" /> Generate draft
                  </>
                )}
              </Button>
            </div>
            <Textarea
              id="answer"
              rows={6}
              placeholder="Leave blank and we’ll draft one for you, or write what a strong answer covers."
              aria-invalid={Boolean(errors.answer)}
              {...register('answer')}
            />
            {errors.answer?.message ? (
              <p className="text-xs text-red-300">{errors.answer.message}</p>
            ) : (
              <p className="text-xs text-muted-foreground">
                The AI reviewer compares candidate answers against this.
              </p>
            )}
          </div>

          <div className="grid gap-1.5">
            <Label>Difficulty</Label>
            <div className="flex flex-wrap gap-2">
              {DIFFICULTIES.map((d) => (
                <button
                  key={d}
                  type="button"
                  onClick={() => setValue('difficulty', d, { shouldDirty: true })}
                  className={cn(
                    'rounded-md border px-3 py-1.5 text-xs font-medium uppercase tracking-wide transition-colors',
                    difficulty === d
                      ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                      : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                  )}
                >
                  {d}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-3">
            <Label>Categories (optional)</Label>
            <ChipGroup
              title="Roles"
              items={roles.map((c) => ({ slug: c.slug, name: c.name }))}
              selected={selectedSlugs}
              onToggle={toggleSlug}
              tone="role"
            />
            <ChipGroup
              title="Topics"
              items={topics.map((c) => ({ slug: c.slug, name: c.name }))}
              selected={selectedSlugs}
              onToggle={toggleSlug}
              tone="topic"
            />
          </div>

          {conflict && (
            <ConflictPanel
              matches={conflict}
              onPick={() => onClose(false)}
              onForce={() => submit(getValues(), true)}
              busy={create.isPending}
            />
          )}

          {submitErr && (
            <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
              {submitErr}
            </p>
          )}

          <DialogFooter className="pt-2">
            <DialogClose asChild>
              <Button type="button" variant="ghost">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" variant="brand" disabled={isSubmitting || create.isPending}>
              {isSubmitting || create.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" /> Saving…
                </>
              ) : (
                <>
                  <Plus className="size-4" /> Save question
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// SimilarPanel renders the live (above-warn-threshold) matches under the title
// field. Clicking "Use this" closes the dialog and navigates to that question.
function SimilarPanel({
  matches,
  onPick,
}: {
  matches: SimilarQuestion[]
  onPick: () => void
}) {
  return (
    <div className="rounded-md border border-amber-500/40 bg-amber-500/5 px-3 py-2.5">
      <div className="mb-1.5 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-amber-200">
        <AlertTriangle className="size-3.5" /> Looks similar to existing question{matches.length > 1 ? 's' : ''}
      </div>
      <ul className="space-y-2">
        {matches.slice(0, 4).map((m) => (
          <li
            key={m.id}
            className="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:justify-between sm:gap-2"
          >
            <span className="min-w-0 truncate text-foreground/90">{m.title}</span>
            <span className="flex shrink-0 items-center gap-2">
              <span className="text-xs text-muted-foreground">
                {Math.round(m.similarity * 100)}% match
              </span>
              <Link
                to={`/questions/${m.id}`}
                onClick={onPick}
                className="rounded-full border border-border/60 px-2 py-0.5 text-xs text-foreground/90 hover:border-amber-400/60 hover:text-amber-100"
              >
                Use this
              </Link>
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

// ConflictPanel appears after a 409 response. It mirrors SimilarPanel but adds
// an explicit "Create anyway" path that re-submits with `force=true`.
function ConflictPanel({
  matches,
  onPick,
  onForce,
  busy,
}: {
  matches: SimilarQuestion[]
  onPick: () => void
  onForce: () => void
  busy: boolean
}) {
  return (
    <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2.5">
      <div className="mb-1.5 flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-red-200">
        <AlertTriangle className="size-3.5" /> A near-duplicate already exists
      </div>
      <ul className="mb-2 space-y-2">
        {matches.slice(0, 3).map((m) => (
          <li
            key={m.id}
            className="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:justify-between sm:gap-2"
          >
            <span className="min-w-0 truncate text-foreground/90">{m.title}</span>
            <span className="flex shrink-0 items-center gap-2">
              <span className="text-xs text-muted-foreground">
                {Math.round(m.similarity * 100)}% match
              </span>
              <Link
                to={`/questions/${m.id}`}
                onClick={onPick}
                className="rounded-full border border-border/60 px-2 py-0.5 text-xs text-foreground/90 hover:border-brand-400/60 hover:text-brand-100"
              >
                Use existing
              </Link>
            </span>
          </li>
        ))}
      </ul>
      <Button type="button" variant="ghost" size="sm" onClick={onForce} disabled={busy}>
        {busy ? (
          <>
            <Loader2 className="size-3.5 animate-spin" /> Saving…
          </>
        ) : (
          'Create anyway'
        )}
      </Button>
    </div>
  )
}

function Field({
  id,
  label,
  hint,
  error,
  children,
}: {
  id: string
  label: string
  hint?: string
  error?: string
  children: React.ReactNode
}) {
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {error ? (
        <p className="text-xs text-red-300">{error}</p>
      ) : hint ? (
        <p className="text-xs text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  )
}

function ChipGroup({
  title,
  items,
  selected,
  onToggle,
  tone,
}: {
  title: string
  items: { slug: string; name: string }[]
  selected: Set<string>
  onToggle: (slug: string) => void
  tone: 'role' | 'topic'
}) {
  if (items.length === 0) return null
  return (
    <div>
      <div className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground">
        {title}
      </div>
      <div className="flex flex-wrap gap-2">
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
