import { Link } from 'react-router-dom'
import { Sparkles } from 'lucide-react'

import { useUsage } from '@/hooks/queries'
import type { UsageKind } from '@/types'
import { cn } from '@/lib/utils'

// Order matters here: top three are the headline limits surfaced in the
// dropdown. Anything beyond is one click away on /pricing.
const HEADLINE_KINDS: { kind: UsageKind; label: string }[] = [
  { kind: 'recording_review', label: 'AI reviews' },
  { kind: 'mock_basic', label: 'Mock interviews' },
  { kind: 'question_add', label: 'Questions added' },
]

export function UsageWidget({ className }: { className?: string }) {
  const { data, isLoading } = useUsage()

  if (isLoading || !data) {
    return (
      <div className={cn('rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground', className)}>
        Loading usage…
      </div>
    )
  }

  if (data.plan === 'pro') {
    return (
      <div className={cn('rounded-md border border-brand-500/40 bg-brand-500/10 px-3 py-2 text-xs', className)}>
        <div className="flex items-center gap-1.5 font-medium text-brand-200">
          <Sparkles className="size-3.5" />
          Pro — unlimited
        </div>
        {data.planExpiresAt && (
          <div className="mt-0.5 text-[11px] text-muted-foreground">
            Renews {new Date(data.planExpiresAt).toLocaleDateString()}
          </div>
        )}
      </div>
    )
  }

  return (
    <div className={cn('rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-xs', className)}>
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <span className="font-medium text-foreground">This week</span>
        <Link to="/pricing" className="text-[11px] text-brand-300 hover:underline">
          Upgrade
        </Link>
      </div>
      <ul className="space-y-1">
        {HEADLINE_KINDS.map(({ kind, label }) => {
          const row = data.quotas[kind]
          if (!row) return null
          const cap = row.limit > 0 ? row.limit : '∞'
          const exhausted = row.limit > 0 && row.used >= row.limit
          return (
            <li key={kind} className="flex items-center justify-between gap-2">
              <span className="text-muted-foreground">{label}</span>
              <span className={cn('tabular-nums', exhausted ? 'font-semibold text-red-300' : 'text-foreground')}>
                {row.used}/{cap}
              </span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
