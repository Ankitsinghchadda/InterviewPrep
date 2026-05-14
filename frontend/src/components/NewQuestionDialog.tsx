import { useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Loader2, Plus } from 'lucide-react'

import { useCategories, useCreateQuestion } from '@/hooks/queries'
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
import type { Difficulty } from '@/types'

const DIFFICULTIES: Difficulty[] = ['easy', 'medium', 'hard']

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
    .min(20, 'A useful reference answer is at least 20 characters.')
    .max(8000, 'Keep the answer under 8000 characters.'),
  difficulty: z.enum(['easy', 'medium', 'hard']),
})

type FormValues = z.infer<typeof schema>

export function NewQuestionDialog() {
  const [open, setOpen] = useState(false)
  const [selectedSlugs, setSelectedSlugs] = useState<Set<string>>(new Set())
  const [submitErr, setSubmitErr] = useState<string | null>(null)

  const { data: categories } = useCategories()
  const create = useCreateQuestion()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
    watch,
    setValue,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { title: '', body: '', answer: '', difficulty: 'medium' },
  })
  const difficulty = watch('difficulty')

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
    }
  }

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSubmitErr(null)
    try {
      await create.mutateAsync({
        title: values.title,
        body: values.body || '',
        answer: values.answer,
        difficulty: values.difficulty,
        categories: Array.from(selectedSlugs),
      })
      onClose(false)
    } catch (err) {
      setSubmitErr((err as Error)?.message || 'Could not save the question.')
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
            Saved to your library. Use it for solo practice or mix it into a mock interview.
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

          <Field
            id="answer"
            label="Reference answer"
            error={errors.answer?.message}
            hint="What a strong answer covers. The AI reviewer compares against this."
          >
            <Textarea
              id="answer"
              rows={6}
              placeholder="A process has its own memory space, scheduled by the OS…"
              aria-invalid={Boolean(errors.answer)}
              {...register('answer')}
            />
          </Field>

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
