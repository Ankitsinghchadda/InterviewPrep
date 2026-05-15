import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import type { CategoryScore, CategoryStrengths } from '@/types'

interface Props {
  data: CategoryStrengths | undefined
  loading: boolean
}

export function CategoryPerformance({ data, loading }: Props) {
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Category performance</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-9 animate-pulse rounded-md bg-card/40" />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  const strong = data?.strong ?? []
  const weak = data?.weak ?? []
  const isEmpty = strong.length === 0 && weak.length === 0

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Category performance</CardTitle>
        <CardDescription>
          Where you're scoring well — and where to focus next. Categories with at least 2 submissions.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isEmpty ? (
          <div className="rounded-md border border-dashed border-border/40 p-6 text-center text-sm text-muted-foreground">
            Try a few questions across topics — performance breakdown appears once you have 2+ submissions per category.
          </div>
        ) : (
          <div className="grid gap-6 md:grid-cols-2">
            <CategoryColumn title="Strong areas" tone="strong" rows={strong} emptyText="No strong areas yet" />
            <CategoryColumn title="Needs work" tone="weak" rows={weak} emptyText="No weak areas yet — keep going" />
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function CategoryColumn({
  title,
  tone,
  rows,
  emptyText,
}: {
  title: string
  tone: 'strong' | 'weak'
  rows: CategoryScore[]
  emptyText: string
}) {
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      {rows.length === 0 ? (
        <div className="rounded-md border border-dashed border-border/40 p-3 text-xs text-muted-foreground">
          {emptyText}
        </div>
      ) : (
        <ul className="space-y-2">
          {rows.map((row) => (
            <CategoryBar key={row.slug} row={row} tone={tone} />
          ))}
        </ul>
      )}
    </div>
  )
}

function CategoryBar({ row, tone }: { row: CategoryScore; tone: 'strong' | 'weak' }) {
  const pct = Math.max(0, Math.min(100, Math.round(row.avgScore)))
  const barColor =
    tone === 'strong'
      ? 'bg-emerald-400'
      : pct < 50
        ? 'bg-red-400'
        : 'bg-amber-400'

  return (
    <li>
      <div className="flex items-center justify-between gap-2 text-sm">
        <span className="truncate font-medium capitalize">{row.name}</span>
        <span className="shrink-0 font-mono text-xs text-muted-foreground">
          {pct}% · {row.submissions}
        </span>
      </div>
      <div className="mt-1 h-2 overflow-hidden rounded-full bg-muted">
        <div className={cn('h-full rounded-full transition-all', barColor)} style={{ width: `${pct}%` }} />
      </div>
    </li>
  )
}
