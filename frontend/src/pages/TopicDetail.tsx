import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, FileQuestion, Loader2 } from 'lucide-react'

import { usePublicCategory } from '@/hooks/queries'
import { useSEO } from '@/hooks/useSEO'
import type { Difficulty, Question } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const DIFFICULTY_VARIANT: Record<Difficulty, 'success' | 'brand' | 'destructive'> = {
  easy: 'success',
  medium: 'brand',
  hard: 'destructive',
}

// TopicDetail is the public landing page for a single topic/role. This is the
// highest-value SEO surface — pages here target searches like "javascript
// interview questions" which have far more search volume than any single
// question. The page lists every public question in the topic so the keyword
// + question titles all live in the indexed HTML.
export function TopicDetail() {
  const { slug } = useParams<{ slug: string }>()
  const { data, isLoading, error } = usePublicCategory(slug)

  const category = data?.category
  const questions = data?.questions ?? []

  const title = category
    ? `${category.name} Interview Questions`
    : 'Topic'
  const description = category
    ? `${questions.length}+ ${category.name} interview questions with reference answers and AI-graded feedback. ${category.description || ''}`.trim().slice(0, 158)
    : 'Browse interview questions by topic.'

  useSEO({
    title,
    description,
    path: `/topics/${slug ?? ''}`,
    type: 'website',
    jsonLd: category
      ? {
          '@context': 'https://schema.org',
          '@type': 'CollectionPage',
          name: title,
          description,
          url: `https://10xinterview.com/topics/${category.slug}`,
          mainEntity: {
            '@type': 'ItemList',
            numberOfItems: questions.length,
            itemListElement: questions.slice(0, 50).map((q, i) => ({
              '@type': 'ListItem',
              position: i + 1,
              url: `https://10xinterview.com/questions/${q.slug ?? q.id}`,
              name: q.title,
            })),
          },
        }
      : undefined,
  })

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading topic…
      </div>
    )
  }

  if (error || !category) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Topic not found</CardTitle>
          <CardDescription>
            The topic you’re looking for doesn’t exist or has been renamed.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant="outline">
            <Link to="/">
              <ArrowLeft className="size-4" /> Home
            </Link>
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <article className="space-y-6">
      <header className="space-y-3">
        <Link
          to="/"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-4" /> Home
        </Link>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl md:text-4xl">
          {category.name} Interview Questions
        </h1>
        {category.description && (
          <p className="text-sm text-muted-foreground sm:text-base">
            {category.description}
          </p>
        )}
        <p className="text-sm text-muted-foreground">
          {questions.length} question{questions.length === 1 ? '' : 's'} — practice with reference
          answers and get AI-graded feedback on your spoken responses.
        </p>
      </header>

      {questions.length === 0 ? (
        <Card>
          <CardHeader>
            <div className="mb-2 inline-flex size-10 items-center justify-center rounded-lg bg-secondary text-muted-foreground">
              <FileQuestion className="size-5" />
            </div>
            <CardTitle>No questions yet in this topic</CardTitle>
            <CardDescription>
              We’re adding more curated questions every week. Check back soon.
            </CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <ul className="space-y-3">
          {questions.map((q) => (
            <li key={q.id}>
              <QuestionListItem q={q} />
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}

function QuestionListItem({ q }: { q: Question }) {
  return (
    <Link to={`/questions/${q.slug ?? q.id}`} className="block">
      <Card className="transition-colors hover:border-brand-500/40 hover:bg-card/80">
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={DIFFICULTY_VARIANT[q.difficulty] || 'outline'}>
              {q.difficulty}
            </Badge>
            {q.categories.slice(0, 3).map((slug) => (
              <Badge key={slug} variant="outline" className="text-muted-foreground">
                {slug}
              </Badge>
            ))}
          </div>
          <CardTitle className="text-base">{q.title}</CardTitle>
          {q.body && (
            <CardDescription className="line-clamp-2">{q.body}</CardDescription>
          )}
        </CardHeader>
      </Card>
    </Link>
  )
}
