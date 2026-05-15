import { Link } from 'react-router-dom'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { LineChart } from 'lucide-react'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { TrendPoint } from '@/types'

interface Props {
  trend: TrendPoint[]
  loading: boolean
}

export function ScoreTrendChart({ trend, loading }: Props) {
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Score trend</CardTitle>
          <CardDescription>Last 30 days</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-[260px] animate-pulse rounded-md bg-card/40" />
        </CardContent>
      </Card>
    )
  }

  const hasData = trend.some((p) => p.avgScore != null)

  return (
    <Card className="h-full">
      <CardHeader>
        <CardTitle className="text-base">Score trend</CardTitle>
        <CardDescription>Daily average over the last 30 days</CardDescription>
      </CardHeader>
      <CardContent>
        {!hasData ? (
          <div className="grid h-[260px] place-items-center rounded-md border border-dashed border-border/40 text-center">
            <div className="flex flex-col items-center gap-2 px-6 text-sm text-muted-foreground">
              <LineChart className="size-6" />
              <p>Your trend appears after your first scored submission.</p>
              <Link to="/questions" className="text-brand-300 hover:underline">
                Practice a question
              </Link>
            </div>
          </div>
        ) : (
          <div className="h-[260px]">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={trend} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="trendFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="currentColor" stopOpacity={0.35} className="text-brand-400" />
                    <stop offset="100%" stopColor="currentColor" stopOpacity={0} className="text-brand-400" />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" className="stroke-border/30" />
                <XAxis
                  dataKey="day"
                  tickFormatter={formatTick}
                  tick={{ fontSize: 11, fill: 'currentColor' }}
                  className="text-muted-foreground"
                  axisLine={false}
                  tickLine={false}
                  minTickGap={20}
                />
                <YAxis
                  domain={[0, 100]}
                  tick={{ fontSize: 11, fill: 'currentColor' }}
                  className="text-muted-foreground"
                  axisLine={false}
                  tickLine={false}
                  width={32}
                />
                <Tooltip content={<TrendTooltip />} />
                <Area
                  type="monotone"
                  dataKey="avgScore"
                  stroke="currentColor"
                  strokeWidth={2}
                  fill="url(#trendFill)"
                  className="text-brand-300"
                  connectNulls
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function formatTick(day: string) {
  // day = YYYY-MM-DD
  const [, m, d] = day.split('-')
  return `${parseInt(m, 10)}/${parseInt(d, 10)}`
}

interface TooltipPayload {
  payload?: TrendPoint
}

function TrendTooltip({ active, payload }: { active?: boolean; payload?: TooltipPayload[] }) {
  if (!active || !payload?.length) return null
  const p = payload[0].payload
  if (!p) return null
  return (
    <div className="rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs shadow-md">
      <div className="font-medium">{p.day}</div>
      <div className="text-muted-foreground">
        {p.avgScore != null ? `Avg ${Math.round(p.avgScore)}/100` : 'No scored submissions'}
      </div>
      <div className="text-muted-foreground">{p.submissions} submission{p.submissions === 1 ? '' : 's'}</div>
    </div>
  )
}
