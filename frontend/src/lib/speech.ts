// Thin wrapper around the browser's Web Speech API for live captioning during
// recording. Returns a stable "final" transcript independent of the always-
// changing "interim" stream so the parent can show both without double-counting.
//
// Browser support: Chrome, Edge, Brave, Opera (Chromium). Safari has it but is
// unreliable for continuous dictation. Firefox doesn't ship it. The recorder
// must gracefully fall back to server STT when isSpeechRecognitionSupported()
// returns false.

type SpeechRecognitionConstructor = new () => SpeechRecognition

// Minimal typing — TS lib.dom.d.ts ships SpeechRecognition only in the latest
// versions; declare the shape we use here to keep this file portable.
interface SpeechRecognition extends EventTarget {
  continuous: boolean
  interimResults: boolean
  lang: string
  start: () => void
  stop: () => void
  abort: () => void
  onresult: ((event: SpeechRecognitionEvent) => void) | null
  onerror: ((event: SpeechRecognitionErrorEvent) => void) | null
  onend: (() => void) | null
}

interface SpeechRecognitionEvent extends Event {
  resultIndex: number
  results: SpeechRecognitionResultList
}

interface SpeechRecognitionResultList {
  length: number
  item(idx: number): SpeechRecognitionResult
  [idx: number]: SpeechRecognitionResult
}

interface SpeechRecognitionResult {
  isFinal: boolean
  length: number
  item(idx: number): SpeechRecognitionAlternative
  [idx: number]: SpeechRecognitionAlternative
}

interface SpeechRecognitionAlternative {
  transcript: string
  confidence: number
}

interface SpeechRecognitionErrorEvent extends Event {
  error: string
  message?: string
}

function getCtor(): SpeechRecognitionConstructor | null {
  const w = window as unknown as {
    SpeechRecognition?: SpeechRecognitionConstructor
    webkitSpeechRecognition?: SpeechRecognitionConstructor
  }
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

export function isSpeechRecognitionSupported(): boolean {
  return getCtor() !== null
}

export interface LiveTranscriberOptions {
  lang?: string
}

export class LiveTranscriber {
  private recognition: SpeechRecognition | null = null
  private finalText = ''
  private interimText = ''
  private onUpdate: ((interim: string, final: string) => void) | null = null
  private shouldRun = false
  private opts: LiveTranscriberOptions

  constructor(opts: LiveTranscriberOptions = {}) {
    this.opts = opts
  }

  start(onUpdate: (interim: string, final: string) => void): boolean {
    const Ctor = getCtor()
    if (!Ctor) return false

    this.finalText = ''
    this.interimText = ''
    this.onUpdate = onUpdate
    this.shouldRun = true

    const r = new Ctor()
    r.continuous = true
    r.interimResults = true
    r.lang = this.opts.lang ?? 'en-US'

    r.onresult = (event) => {
      let interim = ''
      // Process results from resultIndex onward; earlier ones we've already
      // captured into finalText.
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const result = event.results[i]
        const text = result[0]?.transcript ?? ''
        if (result.isFinal) {
          this.finalText += (this.finalText && !this.finalText.endsWith(' ') ? ' ' : '') + text.trim()
        } else {
          interim += text
        }
      }
      this.interimText = interim.trim()
      this.onUpdate?.(this.interimText, this.finalText)
    }

    r.onerror = (event) => {
      // 'no-speech' and 'aborted' are routine; don't crash recording.
      if (event.error === 'no-speech' || event.error === 'aborted') return
      console.warn('SpeechRecognition error:', event.error, event.message)
    }

    r.onend = () => {
      // Chrome stops recognition periodically even with continuous=true. Restart
      // while the caller still wants us running.
      if (this.shouldRun && this.recognition) {
        try {
          this.recognition.start()
        } catch {
          // already running / transitioning — ignore
        }
      }
    }

    this.recognition = r
    try {
      r.start()
      return true
    } catch (err) {
      console.warn('SpeechRecognition start failed:', err)
      this.shouldRun = false
      this.recognition = null
      return false
    }
  }

  // stop ends recognition and returns the assembled final transcript.
  stop(): string {
    this.shouldRun = false
    if (this.recognition) {
      try {
        this.recognition.stop()
      } catch {
        /* ignore */
      }
      this.recognition.onresult = null
      this.recognition.onerror = null
      this.recognition.onend = null
      this.recognition = null
    }
    const out = (this.finalText + (this.interimText ? ' ' + this.interimText : '')).trim()
    return out
  }
}
