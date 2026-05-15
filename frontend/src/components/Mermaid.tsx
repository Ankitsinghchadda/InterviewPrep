import { useEffect, useId, useRef, useState } from 'react'

// Use the fully-bundled ESM build so mermaid doesn't try to lazy-load diagram
// chunks at runtime. The default entry (`mermaid.core.mjs`) splits each diagram
// type into a separate dynamic import — when the chunk URL is intercepted by
// an SPA fallback or proxy it resolves to HTML, producing
// "Expected a JavaScript module script but … MIME type of text/html".
import type MermaidApi from 'mermaid'
// @ts-expect-error -- deep path has no types; runtime shape matches `mermaid`.
import mermaidBundled from 'mermaid/dist/mermaid.esm.min.mjs'
const mermaid = mermaidBundled as typeof MermaidApi

let initialized = false

function ensureInit() {
  if (initialized) return
  mermaid.initialize({
    startOnLoad: false,
    theme: 'dark',
    securityLevel: 'strict',
    fontFamily: 'inherit',
    // Don't append the bomb-icon error SVG to the DOM when the source has a
    // syntax error — we surface the error ourselves in the component below.
    suppressErrorRendering: true,
  })
  initialized = true
}

// Mermaid renders a mermaid diagram from a source string. The library returns
// raw SVG markup we inject; since `securityLevel: 'strict'` is set above,
// mermaid sanitizes node labels itself.
export function Mermaid({ chart }: { chart: string }) {
  const id = useId().replace(/:/g, '')
  const containerRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [svg, setSvg] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    ensureInit()
    setError(null)
    setSvg(null)

    // Parse first so we can show a friendly error without mermaid leaking
    // orphan error nodes into <body>. `suppressErrors: true` makes parse
    // return false instead of throwing on invalid syntax.
    const run = async () => {
      try {
        const parsed = await mermaid.parse(chart, { suppressErrors: true })
        if (cancelled) return
        if (!parsed) {
          setError('Invalid diagram syntax')
          return
        }
        const res = await mermaid.render(`mmd-${id}`, chart)
        if (cancelled) return
        setSvg(res.svg)
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : String(err))
      }
    }
    run()

    return () => {
      cancelled = true
    }
  }, [chart, id])

  if (error) {
    return (
      <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-xs text-red-300">
        Diagram failed to render: {error}
        <pre className="mt-2 whitespace-pre-wrap font-mono text-[11px] text-muted-foreground">
          {chart}
        </pre>
      </div>
    )
  }

  return (
    <div
      ref={containerRef}
      className="my-4 overflow-x-auto rounded-md border border-border bg-card/40 p-3 [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
      // eslint-disable-next-line react/no-danger -- mermaid output is sanitized
      dangerouslySetInnerHTML={svg ? { __html: svg } : undefined}
    >
      {svg ? null : (
        <div className="text-xs text-muted-foreground">Rendering diagram…</div>
      )}
    </div>
  )
}
