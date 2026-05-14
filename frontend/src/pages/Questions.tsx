import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { FileQuestion, Loader2, Trash2, UserCircle2, X } from 'lucide-react'

import { useCategories, useDeleteQuestion, useQuestions } from '@/hooks/queries'
import type { Difficulty, Question } from '@/types'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { NewQuestionDialog } from '@/components/NewQuestionDialog'
import { cn } from '@/lib/utils'

const DIFFICULTIES: Difficulty[] = ['easy', 'medium', 'hard']
const DIFFICULTY_VARIANT: Record<Difficulty, 'success' | 'brand' | 'destructive'> = {
  easy: 'success',
  medium: 'brand',
  hard: 'destructive',
}

export function Questions() {
  const [params, setParams] = useSearchParams()
  const selectedCategories = useMemo(() => {
    const raw = params.get('categories')
    return raw ? raw.split(',').filter(Boolean) : []
  }, [params])
  const difficulty = (params.get('difficulty') as Difficulty | null) || undefined
  const mine = params.get('mine') === 'true'

  const { data: categories } = useCategories()
  const { data: questions, isLoading } = useQuestions({
    categories: selectedCategories,
    difficulty,
    mine,
  })

  const toggleCategory = (slug: string) => {
    const next = new URLSearchParams(params)
    const current = new Set(selectedCategories)
    if (current.has(slug)) current.delete(slug)
    else current.add(slug)
    if (current.size === 0) next.delete('categories')
    else next.set('categories', Array.from(current).join(','))
    setParams(next, { replace: true })
  }

  const setDifficulty = (d?: Difficulty) => {
    const next = new URLSearchParams(params)
    if (d) next.set('difficulty', d)
    else next.delete('difficulty')
    setParams(next, { replace: true })
  }

  const toggleMine = () => {
    const next = new URLSearchParams(params)
    if (mine) next.delete('mine')
    else next.set('mine', 'true')
    setParams(next, { replace: true })
  }

  const clearFilters = () => setParams(new URLSearchParams(), { replace: true })

  const hasFilters = selectedCategories.length > 0 || Boolean(difficulty) || mine
  const rolesAndTopics = categories ?? []

  return (
    <section className="space-y-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Questions</h1>
          <p className="mt-2 text-muted-foreground">
            Tap a question to read the reference answer or practice your own.
          </p>
        </div>
        <NewQuestionDialog />
      </header>

      {/* Filters */}
      <Card className="overflow-hidden">
        <CardContent className="space-y-4 p-5">
          <div className="flex flex-col gap-1.5">
            <FilterLabel>Library</FilterLabel>
            <div className="flex flex-wrap gap-2">
              <FilterChip active={!mine} onClick={() => mine && toggleMine()} variant="topic">
                All questions
              </FilterChip>
              <FilterChip active={mine} onClick={() => !mine && toggleMine()} variant="role">
                <UserCircle2 className="mr-1 inline size-3.5" />
                Mine only
              </FilterChip>
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <FilterLabel>Difficulty</FilterLabel>
            <div className="flex flex-wrap gap-2">
              {DIFFICULTIES.map((d) => (
                <FilterChip
                  key={d}
                  active={difficulty === d}
                  onClick={() => setDifficulty(difficulty === d ? undefined : d)}
                >
                  {d}
                </FilterChip>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <FilterLabel>Categories</FilterLabel>
            <div className="flex flex-wrap gap-2">
              {rolesAndTopics.map((c) => (
                <FilterChip
                  key={c.slug}
                  active={selectedCategories.includes(c.slug)}
                  onClick={() => toggleCategory(c.slug)}
                  variant={c.kind === 'role' ? 'role' : 'topic'}
                >
                  {c.name}
                </FilterChip>
              ))}
              {rolesAndTopics.length === 0 && (
                <span className="text-xs text-muted-foreground">Loading…</span>
              )}
            </div>
          </div>

          {hasFilters && (
            <div className="pt-1">
              <Button size="sm" variant="ghost" onClick={clearFilters}>
                <X className="size-3.5" /> Clear filters
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Results */}
      {isLoading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Loading questions…
        </div>
      ) : (questions ?? []).length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <FileQuestion className="mx-auto mb-3 size-8 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {mine
                ? 'You haven’t added any personal questions yet. Click “New question” to start your library.'
                : 'No questions match these filters.'}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {questions!.map((q) => (
            <QuestionRow key={q.id} question={q} />
          ))}
        </div>
      )}
    </section>
  )
}

function QuestionRow({ question }: { question: Question }) {
  const isMine = Boolean(question.ownerId)
  const del = useDeleteQuestion()

  const onDelete = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!window.confirm(`Delete "${question.title}"? This can't be undone.`)) return
    del.mutate(question.id)
  }

  return (
    <Card className="group transition-colors hover:border-brand-500/40">
      <Link to={`/questions/${question.id}`} className="block">
        <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">{question.title}</CardTitle>
              {isMine && (
                <Badge variant="brand" className="gap-1">
                  <UserCircle2 className="size-3" /> Mine
                </Badge>
              )}
            </div>
            {question.body && (
              <CardDescription className="line-clamp-2">{question.body}</CardDescription>
            )}
            <div className="mt-2 flex flex-wrap gap-1.5">
              {question.categories.slice(0, 5).map((slug) => (
                <Badge key={slug} variant="outline" className="text-muted-foreground">
                  {slug}
                </Badge>
              ))}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Badge variant={DIFFICULTY_VARIANT[question.difficulty] || 'outline'}>
              {question.difficulty}
            </Badge>
            {isMine && (
              <Button
                size="icon"
                variant="ghost"
                onClick={onDelete}
                disabled={del.isPending}
                aria-label="Delete question"
                className="text-muted-foreground opacity-0 transition-opacity hover:text-red-300 group-hover:opacity-100 focus-visible:opacity-100"
              >
                <Trash2 className="size-4" />
              </Button>
            )}
          </div>
        </CardHeader>
      </Link>
    </Card>
  )
}

function FilterLabel({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
      {children}
    </span>
  )
}

function FilterChip({
  active,
  onClick,
  variant = 'topic',
  children,
}: {
  active: boolean
  onClick: () => void
  variant?: 'role' | 'topic'
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'rounded-full border px-3 py-1 text-xs font-medium transition-colors',
        active
          ? variant === 'role'
            ? 'border-brand-400/60 bg-brand-500/20 text-brand-100'
            : 'border-emerald-500/60 bg-emerald-500/15 text-emerald-200'
          : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
      )}
    >
      {children}
    </button>
  )
}
