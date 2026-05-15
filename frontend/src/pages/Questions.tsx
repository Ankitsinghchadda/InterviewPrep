import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  FileQuestion,
  Loader2,
  Search,
  SlidersHorizontal,
  Sparkles,
  Trash2,
  UserCircle2,
  Wand2,
  X,
} from 'lucide-react'

import {
  useCategories,
  useDeleteQuestion,
  useGenerateQuestions,
  useQuestions,
} from '@/hooks/queries'
import type { Difficulty, Question } from '@/types'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { NewQuestionDialog } from '@/components/NewQuestionDialog'
import { QuestionSearchBar } from '@/components/QuestionSearchBar'
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
  const q = params.get('q') ?? ''

  const { data: categories } = useCategories()
  const { data: questions, isLoading, isFetching } = useQuestions({
    categories: selectedCategories,
    difficulty,
    mine,
    q,
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

  // Stable callback so the debounce effect in QuestionSearchBar doesn't see a
  // changing function identity on every render.
  const setQuery = useCallback(
    (next: string) => {
      setParams((prev) => {
        const out = new URLSearchParams(prev)
        if (next) out.set('q', next)
        else out.delete('q')
        return out
      }, { replace: true })
    },
    [setParams],
  )

  const clearFilters = () => setParams(new URLSearchParams(), { replace: true })

  const hasFilters =
    selectedCategories.length > 0 || Boolean(difficulty) || mine || Boolean(q)
  const rolesAndTopics = categories ?? []
  const categoryNames = useMemo(() => {
    const map = new Map<string, string>()
    for (const c of rolesAndTopics) map.set(c.slug, c.name)
    return map
  }, [rolesAndTopics])

  // Build active-filter chips. Each chip clears its own facet on click.
  const activeChips: { key: string; label: React.ReactNode; onClear: () => void }[] = []
  if (q) {
    activeChips.push({
      key: 'q',
      label: (
        <>
          <Search className="size-3" />
          <span className="max-w-[12rem] truncate">{q}</span>
        </>
      ),
      onClear: () => setQuery(''),
    })
  }
  if (mine) {
    activeChips.push({
      key: 'mine',
      label: (
        <>
          <UserCircle2 className="size-3" /> Mine only
        </>
      ),
      onClear: toggleMine,
    })
  }
  if (difficulty) {
    activeChips.push({
      key: 'difficulty',
      label: <span className="capitalize">{difficulty}</span>,
      onClear: () => setDifficulty(undefined),
    })
  }
  for (const slug of selectedCategories) {
    activeChips.push({
      key: `cat-${slug}`,
      label: categoryNames.get(slug) ?? slug,
      onClear: () => toggleCategory(slug),
    })
  }

  const isSearching = Boolean(q)
  const results = questions ?? []

  const PAGE_SIZE = 15
  const [visible, setVisible] = useState(PAGE_SIZE)
  // Reset pagination when filters/search change so users always start at the top.
  useEffect(() => {
    setVisible(PAGE_SIZE)
  }, [q, difficulty, mine, selectedCategories.join(',')])
  const shownResults = results.slice(0, visible)
  const hasMore = results.length > visible

  const generate = useGenerateQuestions()
  const [generateErr, setGenerateErr] = useState<string | null>(null)
  // Only offer AI generation when the user has narrowed to a category and is
  // looking at the shared library — not on a free-text search and not in the
  // "Mine only" view, where the empty state has a different cause.
  const canGenerate =
    selectedCategories.length > 0 && !mine && !isSearching && !generate.isPending
  const selectedCategoryNames = useMemo(
    () =>
      selectedCategories
        .map((slug) => categoryNames.get(slug) ?? slug)
        .join(', '),
    [selectedCategories, categoryNames],
  )

  const onGenerate = () => {
    setGenerateErr(null)
    generate.mutate(
      { categories: selectedCategories, difficulty, count: 5 },
      {
        onError: (err) =>
          setGenerateErr(err.message || 'Could not generate questions.'),
      },
    )
  }

  return (
    <section className="space-y-6">
      <header className="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Questions</h1>
          <p className="mt-2 text-sm text-muted-foreground sm:text-base">
            Tap a question to read the reference answer or practice your own.
          </p>
        </div>
        <NewQuestionDialog />
      </header>

      <div className="flex items-stretch gap-2">
        <QuestionSearchBar value={q} onCommit={setQuery} className="flex-1" />
        <FilterPopover
          activeCount={
            selectedCategories.length + (difficulty ? 1 : 0) + (mine ? 1 : 0)
          }
        >
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
        </FilterPopover>
      </div>

      {isSearching && (
        <p className="-mt-2 flex items-center gap-1.5 px-1 text-xs text-muted-foreground">
          <Sparkles className="size-3 text-brand-300" />
          Semantic search — results ranked by meaning + keyword match.
        </p>
      )}

      {activeChips.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs uppercase tracking-wider text-muted-foreground">
            Active
          </span>
          {activeChips.map((chip) => (
            <button
              key={chip.key}
              type="button"
              onClick={chip.onClear}
              className="inline-flex items-center gap-1.5 rounded-full border border-brand-400/40 bg-brand-500/15 px-2.5 py-1 text-xs text-brand-100 transition-colors hover:bg-brand-500/25"
            >
              {chip.label}
              <X className="size-3 opacity-70" />
            </button>
          ))}
          <Button size="sm" variant="ghost" onClick={clearFilters} className="h-7 px-2 text-xs">
            Clear all
          </Button>
        </div>
      )}

      {/* Results */}
      {isLoading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Loading questions…
        </div>
      ) : results.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <FileQuestion className="mx-auto mb-3 size-8 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {isSearching
                ? `No questions match “${q}”. Try removing a filter or rephrasing.`
                : mine
                  ? 'You haven’t added any personal questions yet. Click “New question” to start your library.'
                  : selectedCategories.length > 0
                    ? `No questions yet for ${selectedCategoryNames}.`
                    : 'No questions match these filters.'}
            </p>
            {canGenerate && (
              <div className="mt-5 flex flex-col items-center gap-2">
                <Button
                  variant="brand"
                  onClick={onGenerate}
                  disabled={generate.isPending}
                >
                  {generate.isPending ? (
                    <>
                      <Loader2 className="size-4 animate-spin" />
                      Asking Gemini for 5 questions…
                    </>
                  ) : (
                    <>
                      <Wand2 className="size-4" />
                      Generate 5 questions with AI
                    </>
                  )}
                </Button>
                <p className="max-w-sm text-xs text-muted-foreground">
                  We&apos;ll ask the AI to draft questions for{' '}
                  <span className="text-foreground">{selectedCategoryNames}</span>
                  {difficulty ? ` at ${difficulty} difficulty` : ''} and add them
                  to the public library. Duplicates of existing titles are
                  skipped.
                </p>
                {generateErr && (
                  <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-1.5 text-xs text-red-300">
                    {generateErr}
                  </p>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      ) : (
        <>
          <p className="px-1 text-xs text-muted-foreground">
            Showing {shownResults.length} of {results.length}{' '}
            {results.length === 1 ? 'question' : 'questions'}
            {isSearching && isFetching && (
              <Loader2 className="ml-1 inline size-3 animate-spin align-text-bottom" />
            )}
          </p>
          <div className="space-y-3">
            {shownResults.map((qu) => (
              <QuestionRow key={qu.id} question={qu} />
            ))}
          </div>
          {hasMore && (
            <div className="flex justify-center pt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setVisible((v) => v + PAGE_SIZE)}
              >
                Show more
              </Button>
            </div>
          )}
          {!hasMore && visible > PAGE_SIZE && (
            <div className="flex justify-center pt-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setVisible(PAGE_SIZE)}
              >
                Show less
              </Button>
            </div>
          )}
        </>
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

  // Prefer the server-supplied snippet (HTML with <mark> from ts_headline)
  // when present; fall back to plain body. ts_headline only emits <mark> tags,
  // and we set MaxFragments=1 so this is bounded — safe to render as HTML.
  const hasSnippet = Boolean(question.snippet)

  return (
    <Card className="group transition-colors hover:border-brand-500/40">
      <Link to={`/questions/${question.id}`} className="block">
        <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0 sm:gap-4">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <CardTitle className="text-base">{question.title}</CardTitle>
              {isMine && (
                <Badge variant="brand" className="gap-1">
                  <UserCircle2 className="size-3" /> Mine
                </Badge>
              )}
            </div>
            {hasSnippet ? (
              <CardDescription
                className="line-clamp-2 [&_mark]:rounded [&_mark]:bg-brand-500/25 [&_mark]:px-0.5 [&_mark]:text-brand-50"
                dangerouslySetInnerHTML={{ __html: question.snippet! }}
              />
            ) : (
              question.body && (
                <CardDescription className="line-clamp-2">{question.body}</CardDescription>
              )
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
                className="text-muted-foreground transition-opacity hover:text-red-300 focus-visible:opacity-100 md:opacity-0 md:group-hover:opacity-100"
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

function FilterPopover({
  activeCount,
  children,
}: {
  activeCount: number
  children: React.ReactNode
}) {
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDocClick = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div ref={wrapRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="dialog"
        className={cn(
          'inline-flex h-full items-center gap-2 rounded-xl border px-3 text-sm transition-colors',
          activeCount > 0 || open
            ? 'border-brand-400/60 bg-brand-500/15 text-brand-100'
            : 'border-border/60 bg-card/40 text-muted-foreground hover:border-border hover:text-foreground',
        )}
      >
        <SlidersHorizontal className="size-4" />
        <span className="hidden sm:inline">Filters</span>
        {activeCount > 0 && (
          <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-brand-500/30 px-1.5 text-xs font-medium text-brand-50">
            {activeCount}
          </span>
        )}
      </button>

      {open && (
        <div
          role="dialog"
          className="absolute right-0 top-[calc(100%+0.5rem)] z-30 w-[min(22rem,calc(100vw-2rem))] space-y-4 rounded-xl border border-border/60 bg-popover p-4 text-popover-foreground shadow-xl sm:w-[26rem]"
        >
          {children}
        </div>
      )}
    </div>
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
