import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Loader2, Plus } from 'lucide-react'

import { useCreateCategory } from '@/hooks/queries'
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
import type { CategoryKind } from '@/types'

const KINDS: CategoryKind[] = ['topic', 'role']

const schema = z.object({
  name: z
    .string()
    .trim()
    .min(2, 'Name should be at least 2 characters.')
    .max(60, 'Keep the name under 60 characters.'),
  slug: z
    .string()
    .trim()
    .min(2, 'Slug should be at least 2 characters.')
    .max(60, 'Keep the slug under 60 characters.')
    .regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, 'Use lowercase letters, digits, and hyphens (kebab-case).'),
  kind: z.enum(['role', 'topic']),
  description: z.string().trim().max(200, 'Keep the description under 200 characters.').optional(),
})

type FormValues = z.infer<typeof schema>

// slugify mirrors what an admin would type by hand: lowercase, hyphenated,
// alphanumeric-only. Auto-fills the slug while the user is still typing the
// name so it feels effortless; once they edit the slug we stop overwriting.
function slugify(name: string): string {
  return name
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
}

export function NewSkillDialog() {
  const [open, setOpen] = useState(false)
  const [slugTouched, setSlugTouched] = useState(false)
  const [submitErr, setSubmitErr] = useState<string | null>(null)
  const create = useCreateCategory()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
    watch,
    setValue,
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', slug: '', kind: 'topic', description: '' },
  })

  const name = watch('name')
  const kind = watch('kind')

  // Auto-fill slug from name until the user starts editing it directly.
  useEffect(() => {
    if (!slugTouched) {
      setValue('slug', slugify(name), { shouldValidate: false })
    }
  }, [name, slugTouched, setValue])

  const onClose = (next: boolean) => {
    setOpen(next)
    if (!next) {
      reset()
      setSlugTouched(false)
      setSubmitErr(null)
    }
  }

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSubmitErr(null)
    try {
      await create.mutateAsync({
        name: values.name,
        slug: values.slug,
        kind: values.kind,
        description: values.description || '',
      })
      onClose(false)
    } catch (err) {
      setSubmitErr((err as Error)?.message || 'Could not create the skill.')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogTrigger asChild>
        <Button variant="brand">
          <Plus className="size-4" /> New skill
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add a new skill</DialogTitle>
          <DialogDescription>
            Skills show up on the Topics page and can be tagged on questions.
            We&apos;ll generate 5 starter interview questions in the background — they&apos;ll appear on the Questions page within a minute.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
          <div className="grid gap-1.5">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              placeholder="GraphQL"
              aria-invalid={Boolean(errors.name)}
              {...register('name')}
            />
            {errors.name?.message && (
              <p className="text-xs text-red-300">{errors.name.message}</p>
            )}
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="slug">Slug</Label>
            <Input
              id="slug"
              placeholder="graphql"
              aria-invalid={Boolean(errors.slug)}
              {...register('slug', { onChange: () => setSlugTouched(true) })}
            />
            {errors.slug?.message ? (
              <p className="text-xs text-red-300">{errors.slug.message}</p>
            ) : (
              <p className="text-xs text-muted-foreground">
                Used in URLs. Auto-filled from the name — edit if you want something different.
              </p>
            )}
          </div>

          <div className="grid gap-1.5">
            <Label>Kind</Label>
            <div className="flex flex-wrap gap-2">
              {KINDS.map((k) => (
                <button
                  key={k}
                  type="button"
                  onClick={() => setValue('kind', k, { shouldDirty: true })}
                  className={cn(
                    'rounded-md border px-3 py-1.5 text-xs font-medium uppercase tracking-wide transition-colors',
                    kind === k
                      ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
                      : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
                  )}
                >
                  {k}
                </button>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              Topics are technologies (e.g., GraphQL). Roles are job profiles (e.g., Backend Engineer).
            </p>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="description">Description (optional)</Label>
            <Textarea
              id="description"
              rows={2}
              placeholder="Schema, resolvers, federation."
              aria-invalid={Boolean(errors.description)}
              {...register('description')}
            />
            {errors.description?.message && (
              <p className="text-xs text-red-300">{errors.description.message}</p>
            )}
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
                  <Loader2 className="size-4 animate-spin" /> Creating…
                </>
              ) : (
                <>
                  <Plus className="size-4" /> Create skill
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
