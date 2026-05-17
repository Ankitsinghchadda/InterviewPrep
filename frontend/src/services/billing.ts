import { api } from './api'
import type { UsageSnapshot } from '@/types'

interface ApiEnvelope<T> {
  data?: T
  error?: string
}

export async function getUsage(): Promise<UsageSnapshot> {
  const { data } = await api.get<ApiEnvelope<UsageSnapshot>>('/billing/usage')
  if (!data.data) throw new Error(data.error || 'failed to load usage')
  return data.data
}

export interface PricingPlan {
  id: 'monthly' | 'biannual'
  label: string
  priceINR: number
  priceUSD: number
  intervalMonths: number
  savingsPct?: number
}

export interface PricingResponse {
  plans: PricingPlan[]
  billingCurrency: 'INR'
}

export async function getPlans(): Promise<PricingPlan[]> {
  const { data } = await api.get<ApiEnvelope<PricingResponse>>('/billing/plans')
  return data.data?.plans ?? []
}

export interface CheckoutSession {
  subscriptionId: string
  keyId: string
  plan: 'monthly' | 'biannual'
}

export async function startCheckout(plan: 'monthly' | 'biannual'): Promise<CheckoutSession> {
  const { data } = await api.post<ApiEnvelope<CheckoutSession>>('/billing/checkout', { plan })
  if (!data.data) throw new Error(data.error || 'failed to start checkout')
  return data.data
}

export interface CancelResult {
  cancelled: boolean
  accessUntil?: string
  subscriptionId?: string
}

export async function cancelSubscription(): Promise<CancelResult> {
  const { data } = await api.post<ApiEnvelope<CancelResult>>('/billing/cancel')
  if (!data.data) throw new Error(data.error || 'failed to cancel')
  return data.data
}
