import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Mic, Square, Trash2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { LiveTranscriber, isSpeechRecognitionSupported } from '@/lib/speech'
import { cn } from '@/lib/utils'

type RecorderState = 'idle' | 'recording' | 'recorded' | 'denied'

export interface RecordingPayload {
  blob: Blob
  /** Final transcript from the browser's Web Speech API, if available. */
  transcript?: string
}

interface RecorderProps {
  /** Called when the user clicks Submit; receives the blob + optional transcript. */
  onSubmit: (payload: RecordingPayload) => void
  /** Disable the Submit button externally (e.g., while uploading). */
  busy?: boolean
  /** Hard cap on recording duration in seconds. */
  maxSeconds?: number
  /** Fired the moment recording actually begins (after mic permission). */
  onRecordingStart?: () => void
}

const PREFERRED_TYPES = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']

function pickMimeType(): string {
  for (const t of PREFERRED_TYPES) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(t)) {
      return t
    }
  }
  return ''
}

export function Recorder({ onSubmit, busy = false, maxSeconds = 90, onRecordingStart }: RecorderProps) {
  const [state, setState] = useState<RecorderState>('idle')
  const [elapsed, setElapsed] = useState(0)
  const [blob, setBlob] = useState<Blob | null>(null)
  const [blobUrl, setBlobUrl] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const [liveInterim, setLiveInterim] = useState('')
  const [liveFinal, setLiveFinal] = useState('')
  const [finalTranscript, setFinalTranscript] = useState<string | undefined>(undefined)

  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const audioCtxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const animFrameRef = useRef<number | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const tickRef = useRef<number | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const startedAtRef = useRef<number | null>(null)
  const transcriberRef = useRef<LiveTranscriber | null>(null)
  const maxSecondsRef = useRef(maxSeconds)
  useEffect(() => {
    maxSecondsRef.current = maxSeconds
  }, [maxSeconds])

  const speechSupported = isSpeechRecognitionSupported()

  const cleanup = useCallback(() => {
    if (animFrameRef.current) cancelAnimationFrame(animFrameRef.current)
    animFrameRef.current = null
    if (tickRef.current) window.clearInterval(tickRef.current)
    tickRef.current = null
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    if (audioCtxRef.current && audioCtxRef.current.state !== 'closed') {
      void audioCtxRef.current.close()
    }
    audioCtxRef.current = null
    analyserRef.current = null
  }, [])

  useEffect(() => () => cleanup(), [cleanup])
  useEffect(() => {
    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl)
    }
  }, [blobUrl])

  const drawWaveform = useCallback(() => {
    const canvas = canvasRef.current
    const analyser = analyserRef.current
    if (!canvas || !analyser) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const width = canvas.width
    const height = canvas.height
    const buf = new Uint8Array(analyser.frequencyBinCount)

    const render = () => {
      animFrameRef.current = requestAnimationFrame(render)
      analyser.getByteFrequencyData(buf)
      ctx.clearRect(0, 0, width, height)

      const bars = 48
      const barWidth = width / bars
      for (let i = 0; i < bars; i++) {
        const slice = Math.floor((i / bars) * buf.length)
        const v = buf[slice] ?? 0
        const h = Math.max(2, (v / 255) * height)
        const grad = ctx.createLinearGradient(0, height, 0, height - h)
        grad.addColorStop(0, 'rgba(99, 102, 241, 0.95)') // brand bottom
        grad.addColorStop(1, 'rgba(217, 70, 239, 0.95)') // fuchsia top
        ctx.fillStyle = grad
        const x = i * barWidth + 1
        ctx.fillRect(x, height - h, barWidth - 2, h)
      }
    }
    render()
  }, [])

  const startRecording = useCallback(async () => {
    setErrorMsg(null)
    if (blobUrl) {
      URL.revokeObjectURL(blobUrl)
      setBlobUrl(null)
    }
    setBlob(null)
    setFinalTranscript(undefined)
    setLiveInterim('')
    setLiveFinal('')
    chunksRef.current = []
    setElapsed(0)

    let stream: MediaStream
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        // Speech-optimized constraints: mono is half the bytes and STT models
        // are tuned for mono; echo/noise filters improve transcript accuracy.
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      })
    } catch (err) {
      console.error('mic access denied', err)
      setState('denied')
      setErrorMsg('Microphone access was blocked. Allow it in your browser settings and try again.')
      return
    }
    streamRef.current = stream

    // Visualizer
    try {
      const audioCtx = new AudioContext()
      audioCtxRef.current = audioCtx
      const source = audioCtx.createMediaStreamSource(stream)
      const analyser = audioCtx.createAnalyser()
      analyser.fftSize = 256
      source.connect(analyser)
      analyserRef.current = analyser
      drawWaveform()
    } catch (err) {
      // Visualizer is nice-to-have; recording continues without it.
      console.warn('visualizer failed', err)
    }

    // MediaRecorder — drop to ~48 kbps opus. Chrome's default is ~128 kbps
    // which is overkill for speech; 48 kbps opus is indistinguishable for
    // transcription and produces uploads ~3x smaller (a 90s answer is ~540KB
    // instead of ~1.4MB).
    const mimeType = pickMimeType()
    const recorderOpts: MediaRecorderOptions = { audioBitsPerSecond: 48_000 }
    if (mimeType) recorderOpts.mimeType = mimeType
    let recorder: MediaRecorder
    try {
      recorder = new MediaRecorder(stream, recorderOpts)
    } catch (err) {
      console.error('MediaRecorder failed', err)
      cleanup()
      setState('denied')
      setErrorMsg('This browser doesn’t support audio recording.')
      return
    }
    mediaRecorderRef.current = recorder

    recorder.ondataavailable = (e) => {
      if (e.data.size > 0) chunksRef.current.push(e.data)
    }
    recorder.onstop = () => {
      const finalType = recorder.mimeType || mimeType || 'audio/webm'
      const finalBlob = new Blob(chunksRef.current, { type: finalType })
      const url = URL.createObjectURL(finalBlob)
      // Stop Web Speech and capture the final transcript (if supported).
      if (transcriberRef.current) {
        const t = transcriberRef.current.stop()
        transcriberRef.current = null
        setFinalTranscript(t || undefined)
      }
      setBlob(finalBlob)
      setBlobUrl(url)
      setState('recorded')
      cleanup()
    }

    // Web Speech: live captions in parallel with MediaRecorder. Browsers
    // without support gracefully fall through to server STT after upload.
    if (speechSupported) {
      const t = new LiveTranscriber()
      const started = t.start((interim, final) => {
        setLiveInterim(interim)
        setLiveFinal(final)
      })
      if (started) transcriberRef.current = t
    }

    recorder.start(250) // emit chunks every 250ms for low memory pressure
    startedAtRef.current = performance.now()
    tickRef.current = window.setInterval(() => {
      if (!startedAtRef.current) return
      const secs = Math.floor((performance.now() - startedAtRef.current) / 1000)
      setElapsed(secs)
      if (secs >= maxSecondsRef.current) {
        recorder.stop()
      }
    }, 200)

    setState('recording')
    onRecordingStart?.()
  }, [blobUrl, cleanup, drawWaveform, speechSupported, onRecordingStart])

  const stopRecording = useCallback(() => {
    mediaRecorderRef.current?.stop()
  }, [])

  const discard = useCallback(() => {
    if (blobUrl) URL.revokeObjectURL(blobUrl)
    setBlobUrl(null)
    setBlob(null)
    setFinalTranscript(undefined)
    setLiveInterim('')
    setLiveFinal('')
    setElapsed(0)
    setState('idle')
  }, [blobUrl])

  const submit = useCallback(() => {
    if (blob) onSubmit({ blob, transcript: finalTranscript })
  }, [blob, finalTranscript, onSubmit])

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-border/60 bg-card/40 p-5">
        <div className="flex items-center gap-4">
          <RecordButton
            state={state}
            onStart={startRecording}
            onStop={stopRecording}
            disabled={busy}
          />
          <div className="flex-1">
            <div className="flex items-baseline justify-between gap-2">
              <span className="font-mono text-sm tabular-nums text-foreground">
                {fmtTime(elapsed)} / {fmtTime(maxSeconds)}
              </span>
              <StateLabel state={state} />
            </div>
            <div className="relative mt-2 h-12 overflow-hidden rounded-md bg-background/40">
              <canvas
                ref={canvasRef}
                width={600}
                height={48}
                className={cn(
                  'h-full w-full',
                  state === 'recording' ? 'opacity-100' : 'opacity-30',
                )}
              />
              {state !== 'recording' && (
                <div className="pointer-events-none absolute inset-0 flex items-center justify-center text-xs text-muted-foreground">
                  {state === 'recorded' ? 'Playback below — submit when ready.' : 'Press the mic to start.'}
                </div>
              )}
            </div>
          </div>
        </div>

        {speechSupported && (state === 'recording' || (state === 'recorded' && (liveFinal || finalTranscript))) && (
          <LiveCaption
            interim={liveInterim}
            final={state === 'recorded' ? (finalTranscript ?? liveFinal) : liveFinal}
          />
        )}

        {state === 'recorded' && blobUrl && (
          <div className="mt-4 flex items-center gap-3">
            <PlaybackAudio src={blobUrl} />
            <Button variant="ghost" size="sm" onClick={discard} disabled={busy}>
              <Trash2 className="size-4" /> Re-record
            </Button>
          </div>
        )}

        {errorMsg && (
          <p className="mt-3 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-red-300">
            {errorMsg}
          </p>
        )}
      </div>

      <div className="flex justify-end">
        <Button
          variant="brand"
          size="lg"
          onClick={submit}
          disabled={!blob || busy}
        >
          {busy ? 'Submitting…' : 'Submit answer'}
        </Button>
      </div>
    </div>
  )
}

// LiveCaption renders the live Web Speech transcript while the user is
// speaking. Final words appear in the foreground colour; the in-flight interim
// candidate appears dimmer because it can still change. The container auto-
// scrolls to the latest text on every update so long answers don't visually
// stall — the candidate sees their words landing without having to scroll.
function LiveCaption({ interim, final }: { interim: string; final: string }) {
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const sentinelRef = useRef<HTMLDivElement | null>(null)
  // useLayoutEffect + rAF: ensures the DOM is fully laid out before we read
  // scrollHeight. Safari occasionally returns stale heights from a plain
  // useEffect, so we anchor a sentinel at the bottom and scroll to it.
  useLayoutEffect(() => {
    const el = scrollRef.current
    const sentinel = sentinelRef.current
    if (!el || !sentinel) return
    const id = requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight
    })
    return () => cancelAnimationFrame(id)
  }, [interim, final])

  if (!interim && !final) {
    return (
      <div className="mt-3 rounded-md border border-border/40 bg-background/30 px-3 py-2 text-xs text-muted-foreground italic">
        Listening… speak naturally; live captions will appear here.
      </div>
    )
  }
  return (
    <div
      ref={scrollRef}
      className="mt-3 max-h-60 overflow-y-auto rounded-md border border-border/40 bg-background/30 px-3 py-2 text-sm leading-relaxed"
    >
      <span className="text-foreground">{final}</span>
      {final && interim ? ' ' : ''}
      <span className="text-muted-foreground">{interim}</span>
      <div ref={sentinelRef} aria-hidden />
    </div>
  )
}

// PlaybackAudio wraps <audio> with a workaround for the webm/MediaRecorder
// "duration is Infinity / wrong" bug. Chrome and Safari emit webm without
// proper duration metadata; the scrubber jumps to the end after a few seconds
// and then idles. Seeking to a huge currentTime forces the browser to scan the
// whole blob and recompute duration to the real value — once it lands we
// reset to 0 so the bar is ready to play normally.
function PlaybackAudio({ src }: { src: string }) {
  const ref = useRef<HTMLAudioElement | null>(null)

  useEffect(() => {
    const audio = ref.current
    if (!audio) return
    let cancelled = false

    const onLoaded = () => {
      if (cancelled) return
      if (audio.duration === Infinity || isNaN(audio.duration)) {
        const onTimeUpdate = () => {
          if (cancelled) return
          audio.removeEventListener('timeupdate', onTimeUpdate)
          audio.currentTime = 0
        }
        audio.addEventListener('timeupdate', onTimeUpdate)
        // Seeking far past the end triggers a full-blob scan in Chromium/WebKit.
        audio.currentTime = 1e9
      }
    }

    audio.addEventListener('loadedmetadata', onLoaded)
    // If metadata is already loaded (cached), kick it off manually.
    if (audio.readyState >= 1) onLoaded()

    return () => {
      cancelled = true
      audio.removeEventListener('loadedmetadata', onLoaded)
    }
  }, [src])

  return <audio ref={ref} src={src} controls preload="metadata" className="h-9 w-full" />
}

function RecordButton({
  state,
  onStart,
  onStop,
  disabled,
}: {
  state: RecorderState
  onStart: () => void
  onStop: () => void
  disabled?: boolean
}) {
  const recording = state === 'recording'
  return (
    <button
      type="button"
      onClick={recording ? onStop : onStart}
      disabled={disabled}
      aria-label={recording ? 'Stop recording' : 'Start recording'}
      className={cn(
        'grid size-14 shrink-0 place-items-center rounded-full text-white shadow-lg transition-all',
        'outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
        recording
          ? 'bg-gradient-to-b from-red-500 to-red-700 shadow-red-700/40 hover:scale-105'
          : 'bg-gradient-to-b from-brand-400 to-brand-700 shadow-brand-700/40 hover:scale-105',
        disabled && 'cursor-not-allowed opacity-50',
      )}
    >
      {recording ? <Square className="size-5 fill-current" /> : <Mic className="size-6" />}
      {recording && (
        <span aria-hidden className="absolute inline-flex size-14 animate-ping rounded-full bg-red-500/40" />
      )}
    </button>
  )
}

function StateLabel({ state }: { state: RecorderState }) {
  if (state === 'recording')
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-red-300">
        <span className="size-1.5 animate-pulse rounded-full bg-red-400" /> Recording
      </span>
    )
  if (state === 'recorded')
    return <span className="text-xs font-medium text-emerald-300">Ready to submit</span>
  if (state === 'denied') return <span className="text-xs font-medium text-red-300">Mic blocked</span>
  return <span className="text-xs font-medium text-muted-foreground">Idle</span>
}

function fmtTime(seconds: number) {
  const m = Math.floor(seconds / 60)
    .toString()
    .padStart(2, '0')
  const s = (seconds % 60).toString().padStart(2, '0')
  return `${m}:${s}`
}
