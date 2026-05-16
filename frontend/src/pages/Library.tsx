import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  Bookmark,
  ChevronRight,
  History as HistoryIcon,
  Loader2,
  Plus,
  Star,
  Trash2,
} from 'lucide-react'

import {
  useCollections,
  useDeleteCollection,
  useQuestions,
} from '@/hooks/queries'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/services/api'
import type { ApiEnvelope, Collection, Question, Submission } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CreateCollectionDialog } from '@/components/SaveToCollectionMenu'
import { cn } from '@/lib/utils'

type Tab = 'collections' | 'saved' | 'history'

export function Library() {
  const [params, setParams] = useSearchParams()
  const initial = (params.get('tab') as Tab) || 'collections'
  const [tab, setTab] = useState<Tab>(initial)

  const setTabAndUrl = (next: Tab) => {
    setTab(next)
    const sp = new URLSearchParams(params)
    sp.set('tab', next)
    sp.delete('collection') // drilled-in view is tab-local
    setParams(sp, { replace: true })
  }

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Your library</h1>
        <p className="text-sm text-muted-foreground sm:text-base">
          Save questions into collections, jump back to your bookmarks, and revisit your past attempts.
        </p>
      </header>

      <Tabs value={tab} onValueChange={(v) => setTabAndUrl(v as Tab)}>
        <TabsList>
          <TabsTrigger value="collections">
            <Bookmark className="size-4" /> Collections
          </TabsTrigger>
          <TabsTrigger value="saved">
            <Star className="size-4" /> Saved
          </TabsTrigger>
          <TabsTrigger value="history">
            <HistoryIcon className="size-4" /> History
          </TabsTrigger>
        </TabsList>

        <TabsContent value="collections" className="mt-5">
          <CollectionsTab />
        </TabsContent>
        <TabsContent value="saved" className="mt-5">
          <SavedTab />
        </TabsContent>
        <TabsContent value="history" className="mt-5">
          <HistoryTab />
        </TabsContent>
      </Tabs>
    </section>
  )
}

// ---- Collections tab ------------------------------------------------------

function CollectionsTab() {
  const { data: collections = [], isLoading } = useCollections()
  const [params, setParams] = useSearchParams()
  const drilledId = params.get('collection')
  const drilled = drilledId ? collections.find((c) => c.id === drilledId) : null

  const [createOpen, setCreateOpen] = useState(false)

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading collections…
      </div>
    )
  }

  if (drilled) {
    return (
      <CollectionDetail
        collection={drilled}
        onBack={() => {
          const sp = new URLSearchParams(params)
          sp.delete('collection')
          setParams(sp, { replace: true })
        }}
      />
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {collections.length} {collections.length === 1 ? 'collection' : 'collections'}
        </p>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="size-4" /> New collection
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {collections.map((c) => (
          <CollectionCard
            key={c.id}
            collection={c}
            onOpen={() => {
              const sp = new URLSearchParams(params)
              sp.set('collection', c.id)
              setParams(sp, { replace: true })
            }}
          />
        ))}
      </div>

      <CreateCollectionDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}

function CollectionCard({
  collection,
  onOpen,
}: {
  collection: Collection
  onOpen: () => void
}) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group text-left"
    >
      <Card className="h-full transition-colors group-hover:border-brand-500/40">
        <CardHeader>
          <div className="flex items-start justify-between gap-2">
            <CardTitle className="text-base">{collection.name}</CardTitle>
            {collection.isDefault && (
              <Badge variant="brand" className="gap-1">
                <Star className="size-3" /> Default
              </Badge>
            )}
          </div>
          {collection.description ? (
            <CardDescription className="line-clamp-2">{collection.description}</CardDescription>
          ) : (
            <CardDescription>
              {collection.questionCount === 0
                ? 'No questions yet.'
                : `${collection.questionCount} ${
                    collection.questionCount === 1 ? 'question' : 'questions'
                  }`}
            </CardDescription>
          )}
        </CardHeader>
      </Card>
    </button>
  )
}

function CollectionDetail({
  collection,
  onBack,
}: {
  collection: Collection
  onBack: () => void
}) {
  const { data: questions = [], isLoading } = useQuestions({
    inCollection: collection.id,
    limit: 100,
  })
  const del = useDeleteCollection()

  const onDelete = () => {
    if (collection.isDefault) return
    if (!window.confirm(`Delete the "${collection.name}" collection? Questions are not removed from the library.`)) return
    del.mutate(collection.id, {
      onSuccess: () => onBack(),
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ChevronRight className="size-4 rotate-180" /> All collections
        </button>
        {!collection.isDefault && (
          <Button
            size="sm"
            variant="ghost"
            onClick={onDelete}
            disabled={del.isPending}
            className="text-muted-foreground hover:text-red-300"
          >
            <Trash2 className="size-4" /> Delete
          </Button>
        )}
      </div>

      <header className="space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-xl font-semibold">{collection.name}</h2>
          {collection.isDefault && (
            <Badge variant="brand" className="gap-1">
              <Star className="size-3" /> Default
            </Badge>
          )}
        </div>
        {collection.description && (
          <p className="text-sm text-muted-foreground">{collection.description}</p>
        )}
      </header>

      {isLoading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Loading…
        </div>
      ) : questions.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            Nothing here yet. Open any question and use the bookmark button to save it to this collection.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {questions.map((q) => (
            <QuestionLink key={q.id} question={q} />
          ))}
        </div>
      )}
    </div>
  )
}

function QuestionLink({ question }: { question: Question }) {
  return (
    <Link
      to={`/questions/${question.id}`}
      className="block rounded-md border border-border/60 bg-background/40 p-3 transition-colors hover:border-brand-500/40 hover:bg-accent/30"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="truncate text-sm font-medium">{question.title}</p>
          {question.body && (
            <p className="line-clamp-2 text-xs text-muted-foreground">{question.body}</p>
          )}
          <div className="flex flex-wrap gap-1.5">
            {question.categories.slice(0, 5).map((slug) => (
              <Badge key={slug} variant="outline" className="text-muted-foreground">
                {slug}
              </Badge>
            ))}
          </div>
        </div>
        <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
      </div>
    </Link>
  )
}

// ---- Saved tab — shortcut to the default collection ----------------------

function SavedTab() {
  const { data: collections = [], isLoading } = useCollections()
  const defaultCol = collections.find((c) => c.isDefault)

  if (isLoading || !defaultCol) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading…
      </div>
    )
  }

  return <CollectionDetail collection={defaultCol} onBack={() => { /* no back button needed */ }} />
}

// ---- History tab — recent submissions across all questions ----------------

function HistoryTab() {
  const { data: submissions = [], isLoading } = useQuery<Submission[]>({
    queryKey: ['submissions', 'mine'],
    queryFn: async () => {
      const { data } = await api.get<ApiEnvelope<Submission[]>>('/submissions')
      return data.data ?? []
    },
    staleTime: 30_000,
  })

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading history…
      </div>
    )
  }

  if (submissions.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          No recorded answers yet. Try practicing on any question to start building history.
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-2">
      {submissions.map((s) => (
        <Link
          key={s.id}
          to={`/questions/${s.questionId}`}
          className={cn(
            'block rounded-md border border-border/60 bg-background/40 p-3 transition-colors',
            'hover:border-brand-500/40 hover:bg-accent/30',
          )}
        >
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <p className="truncate text-sm">
                {s.feedback ? s.feedback : '(in progress)'}
              </p>
              <p className="text-xs text-muted-foreground">
                {new Date(s.createdAt).toLocaleString(undefined, {
                  month: 'short',
                  day: 'numeric',
                  hour: 'numeric',
                  minute: '2-digit',
                })}
              </p>
            </div>
            <Badge variant="outline" className="text-muted-foreground">
              {s.score != null ? `${Math.round(s.score)} / 100` : s.status}
            </Badge>
          </div>
        </Link>
      ))}
    </div>
  )
}
