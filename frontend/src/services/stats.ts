import { api } from './api'
import type { ApiEnvelope, StatsOverview } from '@/types'

export async function getStatsOverview(): Promise<StatsOverview | null> {
  const { data } = await api.get<ApiEnvelope<StatsOverview>>('/stats/overview')
  return data.data ?? null
}
