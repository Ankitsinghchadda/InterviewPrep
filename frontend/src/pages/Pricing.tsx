import { useState } from 'react'
import { Check, Loader2, Sparkles } from 'lucide-react'
import { Link } from 'react-router-dom'

import { useAuth } from '@/auth/AuthContext'
import { useStartCheckout } from '@/hooks/queries'
import { useSEO } from '@/hooks/useSEO'
import { openCheckout } from '@/lib/razorpay'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

interface Plan {
  id: 'monthly' | 'biannual'
  label: string
  priceUSD: number
  priceINR: number
  intervalLabel: string
  savingsUSD?: string
  savingsINR?: string
  highlight?: boolean
}

// Razorpay charges INR (plan currency is fixed on the Razorpay plan). The
// USD figures are display-only for non-IN visitors — their bank converts at
// its own FX + foreign-txn fee, so the statement amount will differ by a
// few percent. Keep priceINR as the source of truth.
const PLANS: Plan[] = [
  {
    id: 'monthly',
    label: 'Pro Monthly',
    priceUSD: 25,
    priceINR: 2075,
    intervalLabel: 'per month',
  },
  {
    id: 'biannual',
    label: 'Pro 6-month',
    priceUSD: 100,
    priceINR: 8300,
    intervalLabel: 'every 6 months',
    savingsUSD: 'Save 33% — equivalent to $16.67/mo',
    savingsINR: 'Save 33% — equivalent to ₹1,383/mo',
    highlight: true,
  },
]

// India detection via the browser's IANA timezone. VPN/traveling users may
// be misclassified — acceptable trade-off vs. shipping IP geolocation.
function detectIndia(): boolean {
  if (typeof window === 'undefined' || typeof Intl === 'undefined') return false
  try {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone
    return tz === 'Asia/Kolkata' || tz === 'Asia/Calcutta'
  } catch {
    return false
  }
}

const formatINR = (n: number) =>
  new Intl.NumberFormat('en-IN', { maximumFractionDigits: 0 }).format(n)

const FEATURES: { label: string; free: string; pro: string }[] = [
  { label: 'AI review of recorded answers', free: '3 / week', pro: 'Unlimited' },
  { label: 'Mock interviews (topic + adaptive)', free: '2 / week', pro: 'Unlimited' },
  { label: 'Live interactive interviews', free: 'Not available', pro: 'Unlimited' },
  { label: 'Add personal questions', free: '5 / week', pro: 'Unlimited' },
  { label: 'AI-generate question packs', free: 'Not available', pro: 'Unlimited' },
  { label: 'AI explanations + answer drafts', free: '10 / week each', pro: 'Unlimited' },
  { label: 'Reference-answer audio (TTS)', free: '10 / week', pro: 'Unlimited' },
  { label: 'Model', free: 'Gemini 2.5 Flash (Vertex)', pro: 'Gemini 3.1 Pro' },
]

export function Pricing() {
  const { user, refresh } = useAuth()
  const isPro = user?.plan === 'pro'
  const startCheckout = useStartCheckout()
  const [pendingPlan, setPendingPlan] = useState<'monthly' | 'biannual' | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const isIndia = detectIndia()

  useSEO({
    title: 'Pricing — 10xInterview Pro: Unlimited AI Mock Interviews',
    description: isIndia
      ? 'Free to try. Upgrade to Pro for unlimited AI-reviewed answers, mock and live interviews, custom question packs, and Gemini 2.5 Pro feedback. ₹2,075/mo or ₹8,300 for 6 months.'
      : 'Free to try. Upgrade to Pro for unlimited AI-reviewed answers, mock and live interviews, custom question packs, and Gemini 2.5 Pro feedback. $25/mo or $100 for 6 months.',
    path: '/pricing',
    jsonLd: {
      '@context': 'https://schema.org',
      '@type': 'Product',
      name: '10xInterview Pro',
      description:
        'Unlimited AI mock interviews, live practice, custom question packs, and answer reviews scored 0–100.',
      brand: { '@type': 'Brand', name: '10xInterview' },
      // Both currencies are exposed so search engines surface the right
      // one by region. Billing currency is always INR; USD is display only.
      offers: [
        {
          '@type': 'Offer',
          name: 'Pro Monthly (USD)',
          price: '25',
          priceCurrency: 'USD',
          url: 'https://10xinterview.com/pricing',
          availability: 'https://schema.org/InStock',
        },
        {
          '@type': 'Offer',
          name: 'Pro Monthly (INR)',
          price: '2075',
          priceCurrency: 'INR',
          url: 'https://10xinterview.com/pricing',
          availability: 'https://schema.org/InStock',
        },
        {
          '@type': 'Offer',
          name: 'Pro 6-month (USD)',
          price: '100',
          priceCurrency: 'USD',
          url: 'https://10xinterview.com/pricing',
          availability: 'https://schema.org/InStock',
        },
        {
          '@type': 'Offer',
          name: 'Pro 6-month (INR)',
          price: '8300',
          priceCurrency: 'INR',
          url: 'https://10xinterview.com/pricing',
          availability: 'https://schema.org/InStock',
        },
      ],
    },
  })

  const onUpgrade = async (plan: 'monthly' | 'biannual') => {
    if (isPro || pendingPlan) return
    setErrorMsg(null)
    setPendingPlan(plan)
    try {
      const session = await startCheckout.mutateAsync(plan)
      await openCheckout({
        key: session.keyId,
        subscription_id: session.subscriptionId,
        name: '10xInterview',
        description: plan === 'monthly' ? 'Pro Monthly' : 'Pro 6-month',
        prefill: {
          name: user?.name,
          email: user?.email,
        },
        theme: { color: '#7c3aed' },
        handler: () => {
          // Source of truth is the webhook. Refresh /auth/me so the UI
          // reflects the pending upgrade — backend may still be waiting
          // for subscription.activated, but the user shouldn't be left
          // on a stale "Free" badge after they paid.
          void refresh()
        },
        modal: {
          ondismiss: () => setPendingPlan(null),
        },
      })
    } catch (err) {
      const e = err as {
        response?: { status?: number; data?: { error?: string } }
        message?: string
      }
      const code = e.response?.data?.error
      let msg: string
      if (e.response?.status === 503 || code === 'payments_not_configured') {
        msg = 'CHECKOUT_LAUNCHING_SOON'
      } else if (code === 'plan_id not configured') {
        msg = 'This plan isn’t live yet. Please pick the other plan or try again later.'
      } else {
        msg = e.message || 'Could not start checkout.'
      }
      setErrorMsg(msg)
      setPendingPlan(null)
    }
  }

  return (
    <section className="space-y-8 sm:space-y-12">
      <header className="space-y-2 text-center">
        <div className="inline-flex items-center gap-1 rounded-full border border-brand-500/40 bg-brand-500/10 px-3 py-1 text-xs font-medium text-brand-200">
          <Sparkles className="size-3.5" /> Pricing
        </div>
        <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">Practice without a ceiling</h1>
        <p className="mx-auto max-w-2xl text-sm text-muted-foreground sm:text-base">
          Free is enough to try the platform; Pro unlocks unlimited AI reviews on the premium model,
          interactive live interviews, and unlimited mock sessions.
        </p>
      </header>

      {errorMsg && (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300">
          {errorMsg === 'CHECKOUT_LAUNCHING_SOON' ? (
            <>
              Checkout is launching soon. For early access,{' '}
              <Link to="/contact" className="underline hover:text-red-200">
                get in touch
              </Link>
              .
            </>
          ) : (
            errorMsg
          )}
        </p>
      )}

      <div className="grid gap-4 sm:grid-cols-2">
        {PLANS.map((p) => (
          <Card
            key={p.id}
            className={cn(
              'flex flex-col',
              p.highlight && 'border-brand-500/60 ring-1 ring-brand-500/30',
            )}
          >
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-lg">{p.label}</CardTitle>
                {p.highlight && (
                  <span className="rounded-full bg-brand-500/20 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-brand-200">
                    Best value
                  </span>
                )}
              </div>
              <CardDescription>
                {isIndia ? (
                  <>
                    <span className="text-2xl font-bold text-foreground">
                      ₹{formatINR(p.priceINR)}
                    </span>{' '}
                    <span className="text-sm text-muted-foreground">{p.intervalLabel}</span>
                    <span className="ml-2 text-xs text-muted-foreground">
                      (~${p.priceUSD} USD)
                    </span>
                  </>
                ) : (
                  <>
                    <span className="text-2xl font-bold text-foreground">${p.priceUSD}</span>{' '}
                    <span className="text-sm text-muted-foreground">{p.intervalLabel}</span>
                    <span className="ml-2 text-xs text-muted-foreground">
                      (billed as ₹{formatINR(p.priceINR)} INR)
                    </span>
                  </>
                )}
              </CardDescription>
              {(isIndia ? p.savingsINR : p.savingsUSD) && (
                <p className="text-xs text-brand-300">
                  {isIndia ? p.savingsINR : p.savingsUSD}
                </p>
              )}
            </CardHeader>
            <CardContent className="mt-auto">
              {isPro ? (
                <Button variant="ghost" disabled className="w-full">
                  You're on Pro
                </Button>
              ) : (
                <Button
                  variant={p.highlight ? 'brand' : 'outline'}
                  className="w-full"
                  disabled={pendingPlan !== null}
                  onClick={() => onUpgrade(p.id)}
                >
                  {pendingPlan === p.id ? (
                    <>
                      <Loader2 className="size-4 animate-spin" />
                      Opening checkout…
                    </>
                  ) : (
                    <>Upgrade to {p.label}</>
                  )}
                </Button>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">What you get</CardTitle>
          <CardDescription>Side-by-side: free vs. Pro.</CardDescription>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <table className="w-full min-w-[480px] text-sm">
            <thead>
              <tr className="border-b border-border/60 text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th className="py-2 pr-4 font-medium">Feature</th>
                <th className="py-2 pr-4 font-medium">Free</th>
                <th className="py-2 pr-4 font-medium">Pro</th>
              </tr>
            </thead>
            <tbody>
              {FEATURES.map((f) => (
                <tr key={f.label} className="border-b border-border/40 last:border-0">
                  <td className="py-2 pr-4 text-foreground">{f.label}</td>
                  <td className="py-2 pr-4 text-muted-foreground">{f.free}</td>
                  <td className="py-2 pr-4 text-brand-200">
                    <span className="inline-flex items-center gap-1">
                      <Check className="size-3.5 text-brand-300" />
                      {f.pro}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>

      <div className="rounded-md border border-border/60 bg-muted/20 p-4 text-xs text-muted-foreground sm:text-sm">
        {isIndia ? (
          <>
            Payments are processed by Razorpay in INR. Cards, UPI, and netbanking are supported.
            Cancel anytime — access continues through the end of the billing period.
          </>
        ) : (
          <>
            Payments are processed by Razorpay and billed in INR (
            <span className="whitespace-nowrap">~$25 = ₹2,075</span>). International cards are
            accepted; your bank handles the USD↔INR conversion and may add a small foreign
            transaction fee, so your statement may show a slightly different amount. Cancel anytime
            — access continues through the end of the billing period.
          </>
        )}
      </div>

      {!isPro && (
        <div className="text-center text-sm text-muted-foreground">
          Still kicking the tires?{' '}
          <Link to="/dashboard" className="text-brand-300 hover:underline">
            Back to dashboard
          </Link>
        </div>
      )}
    </section>
  )
}
