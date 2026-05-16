import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Sparkles, Lock, ArrowRight } from 'lucide-react'

import { setOnPaywall } from '@/services/api'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { PaywallReason, UsageKind } from '@/types'

// Plain-English copy keyed by UsageKind. Add a row here whenever a new
// billable Kind is registered backend-side.
const KIND_COPY: Record<UsageKind, { title: string; quotaMsg: string; planMsg: string }> = {
  recording_review: {
    title: 'AI review limit reached',
    quotaMsg: 'You’ve used all 3 of your free AI reviews this week. Upgrade for unlimited reviews on the premium model.',
    planMsg: 'AI reviews are available on Pro.',
  },
  mock_basic: {
    title: 'Mock interview limit reached',
    quotaMsg: 'You’ve used both of your free mock interviews this week. Upgrade for unlimited mocks.',
    planMsg: 'Mock interviews are available on Pro.',
  },
  mock_live: {
    title: 'Live interviews are a Pro feature',
    quotaMsg: 'Upgrade to unlock interactive live mock interviews with our premium Gemini model.',
    planMsg: 'Live mock interviews are a Pro-only feature. Upgrade to unlock the interactive interviewer.',
  },
  question_add: {
    title: 'Weekly question limit reached',
    quotaMsg: 'You’ve added your 5 free questions this week. Upgrade for unlimited questions.',
    planMsg: 'Adding questions is available on Pro.',
  },
  question_gen: {
    title: 'Bulk AI question generation is Pro',
    quotaMsg: 'Bulk generation uses a premium model. Upgrade to generate question packs on demand.',
    planMsg: 'Bulk AI question generation is a Pro feature.',
  },
  answer_gen: {
    title: 'AI answer drafting limit reached',
    quotaMsg: 'You’ve hit the weekly cap on standalone AI answer drafts.',
    planMsg: 'AI answer drafting is available on Pro.',
  },
  explanation: {
    title: 'AI explanation limit reached',
    quotaMsg: 'You’ve hit the weekly cap on AI explanations. Upgrade for unlimited explanations on the premium model.',
    planMsg: 'AI explanations are available on Pro.',
  },
  tts: {
    title: 'Audio synthesis limit reached',
    quotaMsg: 'You’ve hit the weekly cap on reference-answer audio. Upgrade for unlimited TTS.',
    planMsg: 'Reference-answer audio is available on Pro.',
  },
}

// PaywallModal is mounted once at the Layout level and listens for global
// 402/403 responses via setOnPaywall. The interceptor (api.ts) pushes the
// most recent reason here so this component owns the "is open" state.
export function PaywallModal() {
  const [reason, setReason] = useState<PaywallReason | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    setOnPaywall((r) => setReason(r))
    return () => setOnPaywall(null)
  }, [])

  const open = reason !== null
  const close = () => setReason(null)
  const onUpgrade = () => {
    close()
    navigate('/pricing')
  }

  const copy = reason ? KIND_COPY[reason.kind] : null
  const isPlanRequired = reason?.error === 'plan_required'

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent>
        <DialogHeader>
          <div className="mb-1 flex items-center gap-2 text-brand-700">
            {isPlanRequired ? <Lock className="size-4" /> : <Sparkles className="size-4" />}
            <span className="text-xs font-medium uppercase tracking-wide">
              {isPlanRequired ? 'Pro feature' : 'Upgrade to Pro'}
            </span>
          </div>
          <DialogTitle>{copy?.title ?? 'Upgrade required'}</DialogTitle>
          <DialogDescription>
            {isPlanRequired ? copy?.planMsg : copy?.quotaMsg}
          </DialogDescription>
        </DialogHeader>
        <div className="rounded-md border border-border/60 bg-muted/40 p-3 text-sm">
          <div className="font-medium">Pro — from $25/month</div>
          <ul className="mt-1 list-disc space-y-0.5 pl-5 text-muted-foreground">
            <li>Unlimited AI reviews on the premium Gemini model</li>
            <li>Unlimited mock interviews — including Live mode</li>
            <li>Unlimited question creation + AI generation</li>
          </ul>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={close}>
            Not now
          </Button>
          <Button variant="brand" onClick={onUpgrade}>
            See Pro plans <ArrowRight className="size-4" />
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
