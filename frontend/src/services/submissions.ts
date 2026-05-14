import { api } from './api'
import type { ApiEnvelope, Submission } from '@/types'

export async function submitAnswer(
  questionId: string,
  audio: Blob,
  transcript?: string,
): Promise<Submission> {
  const form = new FormData()
  const ext = mimeToExt(audio.type)
  form.append('audio', audio, `answer.${ext}`)
  if (transcript) form.append('transcript', transcript)
  const { data } = await api.post<ApiEnvelope<Submission>>(
    `/questions/${questionId}/answer`,
    form,
    { headers: { 'Content-Type': 'multipart/form-data' } },
  )
  if (!data.data) throw new Error(data.error || 'submission failed')
  return data.data
}

export async function getSubmission(id: string): Promise<Submission | null> {
  const { data } = await api.get<ApiEnvelope<Submission>>(`/submissions/${id}`)
  return data.data ?? null
}

export interface SubmissionStreamHandlers {
  onTranscript?: (text: string) => void
  onReviewToken?: (delta: string) => void
  onReviewDone?: (final: Submission) => void
  onError?: (err: string) => void
  onOpen?: () => void
}

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/$/, '')

// streamSubmission opens an EventSource to the SSE endpoint and dispatches
// events to the provided callbacks. Returns an unsubscribe function the caller
// MUST invoke on cleanup (typically in useEffect's cleanup).
export function streamSubmission(
  id: string,
  handlers: SubmissionStreamHandlers,
): () => void {
  const url = `${apiBase}/submissions/${id}/stream`
  // EventSource ships httpOnly auth cookies same-origin; nginx proxies us.
  const es = new EventSource(url, { withCredentials: true })

  es.addEventListener('open', () => handlers.onOpen?.())

  es.addEventListener('transcript', (ev) => {
    handlers.onTranscript?.((ev as MessageEvent).data ?? '')
  })

  es.addEventListener('review_token', (ev) => {
    handlers.onReviewToken?.((ev as MessageEvent).data ?? '')
  })

  es.addEventListener('review_done', (ev) => {
    try {
      const parsed = JSON.parse((ev as MessageEvent).data) as Submission
      handlers.onReviewDone?.(parsed)
    } catch (err) {
      handlers.onError?.('failed to parse final submission: ' + String(err))
    } finally {
      es.close()
    }
  })

  es.addEventListener('error', (ev) => {
    // Distinguish server-emitted "error" events (have .data) from EventSource
    // connection errors (no data, just transport failure).
    const data = (ev as MessageEvent).data
    if (data) {
      handlers.onError?.(String(data))
      es.close()
    }
    // Transport errors trigger built-in retry; no action needed here.
  })

  return () => {
    es.close()
  }
}

function mimeToExt(mime: string): string {
  if (!mime) return 'webm'
  if (mime.includes('webm')) return 'webm'
  if (mime.includes('ogg')) return 'ogg'
  if (mime.includes('mp4') || mime.includes('m4a')) return 'm4a'
  if (mime.includes('wav')) return 'wav'
  if (mime.includes('mpeg')) return 'mp3'
  return 'bin'
}
