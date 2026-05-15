import { Link } from 'react-router-dom'
import { ArrowRight, Sparkles, Target, TrendingUp, TriangleAlert } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { RecommendationItem, SmartRecommendations as SmartRecs } from '@/types'

interface Props {
  recs: SmartRecs | undefined
  loading: boolean
}

export function SmartRecommendations({ recs, loading }: Props) {
  if (loading) {
    return (
      <section className="space-y-3">
        <SectionHeader />
        <div className="h-[200px] animate-pulse rounded-xl border border-border/40 bg-card/40" />
      </section>
    )
  }

  const weak = recs?.weakAreas ?? []
  const level = recs?.levelUp ?? []
  const gaps = recs?.goalGaps ?? []
  const totalCount = weak.length + level.length + gaps.length

  if (totalCount === 0) {
    return (
      <section className="space-y-3">
        <SectionHeader />
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            Practice a few questions and we'll start suggesting where to focus next.
          </CardContent>
        </Card>
      </section>
    )
  }

  // Default to the first bucket that has data so an empty tab isn't shown by default.
  const defaultTab = weak.length > 0 ? 'weak' : level.length > 0 ? 'level' : 'gaps'

  return (
    <section className="space-y-3">
      <SectionHeader />
      <Tabs defaultValue={defaultTab} className="w-full">
        <TabsList className="h-auto flex-wrap p-1">
          <TabsTrigger value="weak" className="gap-1.5" disabled={weak.length === 0}>
            <TriangleAlert className="size-3.5" />
            Shore up weak areas
            <Count n={weak.length} />
          </TabsTrigger>
          <TabsTrigger value="level" className="gap-1.5" disabled={level.length === 0}>
            <TrendingUp className="size-3.5" />
            Level up
            <Count n={level.length} />
          </TabsTrigger>
          <TabsTrigger value="gaps" className="gap-1.5" disabled={gaps.length === 0}>
            <Target className="size-3.5" />
            Goal-aligned gaps
            <Count n={gaps.length} />
          </TabsTrigger>
        </TabsList>
        <TabsContent value="weak" className="mt-4">
          <BucketGrid items={weak} emptyText="No weak areas detected yet." />
        </TabsContent>
        <TabsContent value="level" className="mt-4">
          <BucketGrid items={level} emptyText="Once you're scoring ≥ 80% in a category, level-up suggestions appear here." />
        </TabsContent>
        <TabsContent value="gaps" className="mt-4">
          <BucketGrid items={gaps} emptyText="You've already practiced every category your profile targets." />
        </TabsContent>
      </Tabs>
    </section>
  )
}

function SectionHeader() {
  return (
    <div className="flex items-baseline justify-between">
      <div>
        <h2 className="text-lg font-semibold tracking-tight">What to practice next</h2>
        <p className="text-sm text-muted-foreground">
          Personalized picks based on your performance and goals.
        </p>
      </div>
      <Link to="/questions" className="text-sm text-muted-foreground hover:text-foreground">
        Browse all
      </Link>
    </div>
  )
}

function Count({ n }: { n: number }) {
  if (n === 0) return null
  return (
    <span className="ml-1 rounded-full bg-foreground/10 px-1.5 text-[10px] font-mono">
      {n}
    </span>
  )
}

function BucketGrid({ items, emptyText }: { items: RecommendationItem[]; emptyText: string }) {
  if (items.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border/40 p-6 text-center text-sm text-muted-foreground">
        {emptyText}
      </div>
    )
  }
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {items.map((item) => (
        <RecommendationCard key={item.question.id} item={item} />
      ))}
    </div>
  )
}

function RecommendationCard({ item }: { item: RecommendationItem }) {
  const { question, reason } = item
  return (
    <Link to={`/questions/${question.id}`} className="group">
      <Card className="h-full transition-colors hover:border-brand-500/40">
        <CardHeader>
          <div className="mb-2 flex flex-wrap items-center gap-1.5">
            <Badge variant="brand">{question.difficulty}</Badge>
            {question.categories.slice(0, 3).map((c) => (
              <Badge key={c} variant="outline" className="text-muted-foreground">
                {c}
              </Badge>
            ))}
          </div>
          <CardTitle className="text-base leading-snug">{question.title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-start gap-1.5 text-xs text-muted-foreground">
            <Sparkles className="mt-0.5 size-3 shrink-0 text-brand-300" />
            <span className="flex-1">{reason}</span>
            <ArrowRight className="size-3 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" />
          </div>
        </CardContent>
      </Card>
    </Link>
  )
}
