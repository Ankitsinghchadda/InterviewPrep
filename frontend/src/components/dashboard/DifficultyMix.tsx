import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { DifficultyBucket } from '@/types'

interface Props {
  buckets: DifficultyBucket[] | undefined
  loading: boolean
}

const DIFFICULTY_COLORS: Record<string, string> = {
  easy: 'bg-emerald-400',
  medium: 'bg-amber-400',
  hard: 'bg-red-400',
}

export function DifficultyMix({ buckets, loading }: Props) {
  if (loading) {
    return (
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base">Difficulty mix</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-[120px] animate-pulse rounded-md bg-card/40" />
        </CardContent>
      </Card>
    )
  }

  const rows = buckets ?? []
  const total = rows.reduce((sum, b) => sum + b.submissions, 0)

  return (
    <Card className="h-full">
      <CardHeader>
        <CardTitle className="text-base">Difficulty mix</CardTitle>
        <CardDescription>How your practice splits across levels</CardDescription>
      </CardHeader>
      <CardContent>
        {total === 0 ? (
          <p className="text-sm text-muted-foreground">
            Practice some questions to see your difficulty mix.
          </p>
        ) : (
          <div className="space-y-4">
            <div className="flex h-3 overflow-hidden rounded-full bg-muted">
              {rows.map((b) => (
                <div
                  key={b.difficulty}
                  className={DIFFICULTY_COLORS[b.difficulty] ?? 'bg-muted'}
                  style={{ width: `${(b.submissions / total) * 100}%` }}
                  title={`${b.difficulty}: ${b.submissions}`}
                />
              ))}
            </div>
            <ul className="space-y-1.5 text-sm">
              {(['easy', 'medium', 'hard'] as const).map((d) => {
                const row = rows.find((b) => b.difficulty === d)
                const count = row?.submissions ?? 0
                const avg = row?.avgScore
                return (
                  <li key={d} className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-2">
                      <span className={`size-2.5 rounded-full ${DIFFICULTY_COLORS[d]}`} />
                      <span className="capitalize">{d}</span>
                    </span>
                    <span className="font-mono text-xs text-muted-foreground">
                      {count}
                      {avg != null && count > 0 ? ` · avg ${Math.round(avg)}` : ''}
                    </span>
                  </li>
                )
              })}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
