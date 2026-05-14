import { Link } from 'react-router-dom'
import { Layers, Briefcase, Loader2 } from 'lucide-react'

import { useCategories } from '@/hooks/queries'
import type { Category } from '@/types'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function Topics() {
  const { data, isLoading, error } = useCategories()

  const roles = (data ?? []).filter((c) => c.kind === 'role')
  const topics = (data ?? []).filter((c) => c.kind === 'topic')

  return (
    <section className="space-y-6">
      <header>
        <h1 className="text-3xl font-bold tracking-tight">Topics</h1>
        <p className="mt-2 text-muted-foreground">
          Pick a role for a curated mix, or drill into a specific technology.
        </p>
      </header>

      {error ? (
        <Card>
          <CardHeader>
            <CardTitle>Couldn’t load categories</CardTitle>
            <CardDescription>Try refreshing in a moment.</CardDescription>
          </CardHeader>
        </Card>
      ) : isLoading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Loading categories…
        </div>
      ) : (
        <Tabs defaultValue="roles">
          <TabsList>
            <TabsTrigger value="roles">
              <Briefcase className="size-4" /> Roles ({roles.length})
            </TabsTrigger>
            <TabsTrigger value="topics">
              <Layers className="size-4" /> Topics ({topics.length})
            </TabsTrigger>
          </TabsList>

          <TabsContent value="roles">
            <CategoryGrid items={roles} />
          </TabsContent>
          <TabsContent value="topics">
            <CategoryGrid items={topics} />
          </TabsContent>
        </Tabs>
      )}
    </section>
  )
}

function CategoryGrid({ items }: { items: Category[] }) {
  if (items.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-border/60 px-6 py-10 text-center text-sm text-muted-foreground">
        Nothing here yet.
      </p>
    )
  }
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {items.map((c) => (
        <Link key={c.id} to={`/questions?categories=${encodeURIComponent(c.slug)}`}>
          <Card className="h-full transition-colors hover:border-brand-500/40">
            <CardHeader>
              <div className="mb-2 inline-flex size-9 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                {c.kind === 'role' ? <Briefcase className="size-5" /> : <Layers className="size-5" />}
              </div>
              <CardTitle className="text-base">{c.name}</CardTitle>
              {c.description && <CardDescription>{c.description}</CardDescription>}
            </CardHeader>
          </Card>
        </Link>
      ))}
    </div>
  )
}
