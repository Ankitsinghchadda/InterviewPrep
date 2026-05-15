import { CheckCircle2, TriangleAlert } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { ThemeCount } from '@/types'

interface Props {
  variant: 'strengths' | 'improvements'
  themes: ThemeCount[] | undefined
  loading: boolean
}

const COPY = {
  strengths: {
    title: 'Common strengths',
    description: 'Themes that show up most often in your positive feedback',
    icon: <CheckCircle2 className="size-4 text-emerald-300" />,
    badge: 'success' as const,
    empty: 'Strengths appear after a few submissions.',
  },
  improvements: {
    title: 'Top things to improve',
    description: 'Recurring areas the AI flagged for you to work on',
    icon: <TriangleAlert className="size-4 text-amber-300" />,
    badge: 'destructive' as const,
    empty: 'Improvement areas appear after a few submissions.',
  },
}

export function ThemesList({ variant, themes, loading }: Props) {
  const copy = COPY[variant]

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{copy.title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-[160px] animate-pulse rounded-md bg-card/40" />
        </CardContent>
      </Card>
    )
  }

  const rows = themes ?? []

  return (
    <Card className="h-full">
      <CardHeader>
        <div className="flex items-center gap-2">
          {copy.icon}
          <CardTitle className="text-base">{copy.title}</CardTitle>
        </div>
        <CardDescription>{copy.description}</CardDescription>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">{copy.empty}</p>
        ) : (
          <ul className="flex flex-wrap gap-1.5">
            {rows.map((t) => (
              <li key={t.theme} className="max-w-full">
                <Badge
                  variant={copy.badge}
                  className="max-w-full whitespace-normal text-left capitalize leading-snug"
                >
                  <span className="min-w-0 break-words">{t.theme}</span>
                  <span className="ml-1 shrink-0 font-mono text-[10px] opacity-70">×{t.count}</span>
                </Badge>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
