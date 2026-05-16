import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, Loader2, Star } from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import { useCancelSubscription, useUsage } from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const KIND_LABELS: Record<string, string> = {
  recording_review: 'AI reviews of recorded answers',
  mock_basic: 'Mock interviews (topic + adaptive)',
  mock_live: 'Live interviews',
  question_add: 'Questions added',
  question_gen: 'Bulk AI question generation',
  answer_gen: 'AI answer drafts (standalone)',
  explanation: 'AI explanations',
  tts: 'Reference-answer audio',
}

export function AccountBilling() {
  const { user, refresh } = useAuth()
  const { data: usage } = useUsage()
  const cancel = useCancelSubscription()
  const [confirmOpen, setConfirmOpen] = useState(false)

  const isPro = user?.plan === 'pro'

  const onCancel = async () => {
    try {
      await cancel.mutateAsync()
      setConfirmOpen(false)
      await refresh()
    } catch {
      // useCancelSubscription surfaces the error in cancel.error below
    }
  }

  return (
    <section className="space-y-6 sm:space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Billing</h1>
        <p className="text-sm text-muted-foreground sm:text-base">
          Your plan, renewal date, and how you've used your weekly quota.
        </p>
      </header>

      <Card className={isPro ? 'border-brand-500/40' : undefined}>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="flex items-center gap-2">
                {isPro && <Star className="size-4 text-brand-300" />}
                <CardTitle className="text-base">
                  {isPro ? 'Pro' : 'Free'}
                  {user?.planPeriod === 'monthly' && ' · Monthly'}
                  {user?.planPeriod === 'biannual' && ' · 6-month'}
                </CardTitle>
              </div>
              <CardDescription>
                {isPro && user?.planExpiresAt
                  ? `Renews ${new Date(user.planExpiresAt).toLocaleDateString()}`
                  : isPro
                    ? 'Active'
                    : 'You\'re on the free tier. Upgrade for unlimited AI on the premium Gemini model.'}
              </CardDescription>
            </div>
            <div className="flex flex-col items-end gap-2 sm:flex-row">
              {!isPro && (
                <Button asChild variant="brand">
                  <Link to="/pricing">
                    See Pro plans <ArrowRight className="size-4" />
                  </Link>
                </Button>
              )}
              {isPro && (
                <Button
                  variant="outline"
                  onClick={() => setConfirmOpen(true)}
                  disabled={cancel.isPending}
                >
                  Cancel subscription
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        {cancel.isError && (
          <CardContent>
            <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
              {(cancel.error as Error)?.message || 'Could not cancel. Try again.'}
            </p>
          </CardContent>
        )}
        {cancel.isSuccess && cancel.data?.accessUntil && (
          <CardContent>
            <p className="rounded-md border border-brand-500/40 bg-brand-500/10 px-3 py-2 text-sm text-brand-200">
              Subscription cancelled. You'll have Pro access until{' '}
              {new Date(cancel.data.accessUntil).toLocaleDateString()}.
            </p>
          </CardContent>
        )}
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">This week's usage</CardTitle>
          <CardDescription>
            Rolling 7-day window. Each event drops off exactly 7 days after it happened.
          </CardDescription>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          {!usage ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : (
            <table className="w-full min-w-[420px] text-sm">
              <thead>
                <tr className="border-b border-border/60 text-left text-xs uppercase tracking-wide text-muted-foreground">
                  <th className="py-2 pr-4 font-medium">Action</th>
                  <th className="py-2 pr-4 font-medium">Used</th>
                  <th className="py-2 pr-4 font-medium">Limit</th>
                </tr>
              </thead>
              <tbody>
                {Object.entries(usage.quotas).map(([kind, row]) => (
                  <tr key={kind} className="border-b border-border/40 last:border-0">
                    <td className="py-2 pr-4 text-foreground">
                      {KIND_LABELS[kind] ?? kind}
                    </td>
                    <td className="py-2 pr-4 tabular-nums text-foreground">{row.used}</td>
                    <td className="py-2 pr-4 text-muted-foreground">
                      {row.blocked
                        ? 'Pro only'
                        : row.limit === -1
                          ? 'Unlimited'
                          : row.limit}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Cancel your Pro subscription?</DialogTitle>
            <DialogDescription>
              You'll keep Pro access until the end of your current billing cycle
              {user?.planExpiresAt && ` (${new Date(user.planExpiresAt).toLocaleDateString()})`}.
              After that you'll be moved back to free.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
              Keep Pro
            </Button>
            <Button variant="destructive" onClick={onCancel} disabled={cancel.isPending}>
              {cancel.isPending ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  Cancelling…
                </>
              ) : (
                'Cancel subscription'
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
