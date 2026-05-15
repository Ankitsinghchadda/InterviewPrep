import { Loader2, Sparkles, Volume2 } from 'lucide-react'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { useGenerateExplanation } from '@/hooks/queries'
import { generateQuestionAudio } from '@/services/questions'
import type { Question } from '@/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { lazy, Suspense, useEffect, useState } from 'react'

// Lazy-load Mermaid. The bundled mermaid build is ~3.2MB minified and only
// renders for questions whose explanation contains a ```mermaid``` code block,
// so we don't ship it in the main bundle. The Suspense fallback below shows a
// brief "Rendering diagram…" while the chunk loads on first use per session.
const Mermaid = lazy(() =>
  import('./Mermaid').then((m) => ({ default: m.Mermaid })),
)

// Renders the learner-facing answer for a question: a short conversational
// summary on top and, in a sub-tabbed area below, either the short grader
// reference or the long-form markdown explanation (with optional mermaid
// diagrams). The explanation is generated lazily on first request and
// persisted server-side so subsequent loads are instant.
export function AnswerExplanation({ question }: { question: Question }) {
  const generate = useGenerateExplanation(question.id)
  const hasExplanation = Boolean(question.explanationMarkdown)
  const summary = question.explanationSummary || generate.data?.summary || ''
  const markdown = question.explanationMarkdown || generate.data?.markdown || ''

  return (
    <Card>
      <CardHeader className="flex flex-col items-start justify-between gap-3 space-y-0 sm:flex-row">
        <div className="space-y-1">
          <CardTitle className="text-base">Answer</CardTitle>
          <CardDescription>
            A friendly explanation up top — open <span className="font-medium">Full explanation</span> for the
            deep dive.
          </CardDescription>
        </div>
        <AnswerAudioControl questionId={question.id} audioUrl={question.answerAudioUrl ?? ''} />
      </CardHeader>

      <CardContent className="space-y-6">
        <SummarySection
          summary={summary}
          fallback={question.answer}
          generating={generate.isPending}
          error={generate.error?.message ?? null}
          hasExplanation={hasExplanation || Boolean(generate.data)}
          onGenerate={() => generate.mutate()}
        />

        <Tabs defaultValue="reference">
          <TabsList>
            <TabsTrigger value="reference">Reference</TabsTrigger>
            <TabsTrigger value="full">Full explanation</TabsTrigger>
          </TabsList>

          <TabsContent value="reference">
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
              {question.answer}
            </p>
          </TabsContent>

          <TabsContent value="full">
            <FullExplanationPane
              markdown={markdown}
              generating={generate.isPending}
              error={generate.error?.message ?? null}
              onGenerate={() => generate.mutate()}
            />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}

function SummarySection({
  summary,
  fallback,
  generating,
  error,
  hasExplanation,
  onGenerate,
}: {
  summary: string
  fallback: string
  generating: boolean
  error: string | null
  hasExplanation: boolean
  onGenerate: () => void
}) {
  return (
    <section className="space-y-2">
      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        <Sparkles className="size-3.5 text-brand-300" />
        In simple terms
      </div>
      {summary ? (
        <p className="text-sm leading-relaxed">{summary}</p>
      ) : (
        <>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {fallback}
          </p>
          <div className="flex flex-col gap-1">
            <Button
              variant="outline"
              size="sm"
              onClick={onGenerate}
              disabled={generating}
              className="w-fit"
            >
              {generating ? (
                <>
                  <Loader2 className="size-4 animate-spin" /> Writing the explanation…
                </>
              ) : (
                <>
                  <Sparkles className="size-4" /> Generate friendly explanation
                </>
              )}
            </Button>
            {!generating && !hasExplanation && (
              <span className="text-xs text-muted-foreground">
                One click — generates a plain-English summary plus a deeper writeup with diagrams.
              </span>
            )}
            {error && <span className="text-xs text-red-300">{error}</span>}
          </div>
        </>
      )}
    </section>
  )
}

function FullExplanationPane({
  markdown,
  generating,
  error,
  onGenerate,
}: {
  markdown: string
  generating: boolean
  error: string | null
  onGenerate: () => void
}) {
  if (markdown) {
    return <MarkdownView markdown={markdown} />
  }
  return (
    <div className="flex flex-col items-start gap-3 rounded-md border border-dashed border-border/60 bg-card/30 p-4 text-sm">
      <p className="text-muted-foreground">
        No full explanation generated yet for this question. It’ll include headings, examples, and a diagram
        when the topic calls for one.
      </p>
      <Button variant="outline" size="sm" onClick={onGenerate} disabled={generating}>
        {generating ? (
          <>
            <Loader2 className="size-4 animate-spin" /> Generating…
          </>
        ) : (
          <>
            <Sparkles className="size-4" /> Generate full explanation
          </>
        )}
      </Button>
      {error && <span className="text-xs text-red-300">{error}</span>}
    </div>
  )
}

// Markdown components are styled inline so the explanation matches the rest of
// the dark UI without pulling in @tailwindcss/typography.
const mdComponents: Components = {
  h1: ({ children }) => (
    <h1 className="mt-4 mb-2 text-lg font-semibold tracking-tight">{children}</h1>
  ),
  h2: ({ children }) => (
    <h2 className="mt-4 mb-2 text-base font-semibold tracking-tight">{children}</h2>
  ),
  h3: ({ children }) => (
    <h3 className="mt-3 mb-1 text-sm font-semibold tracking-tight">{children}</h3>
  ),
  p: ({ children }) => <p className="my-2 text-sm leading-relaxed">{children}</p>,
  ul: ({ children }) => (
    <ul className="my-2 ml-5 list-disc space-y-1 text-sm leading-relaxed">{children}</ul>
  ),
  ol: ({ children }) => (
    <ol className="my-2 ml-5 list-decimal space-y-1 text-sm leading-relaxed">{children}</ol>
  ),
  li: ({ children }) => <li className="pl-1">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="my-3 border-l-2 border-brand-500/50 pl-3 text-sm italic text-muted-foreground">
      {children}
    </blockquote>
  ),
  a: ({ children, href }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-brand-300 underline-offset-2 hover:underline"
    >
      {children}
    </a>
  ),
  table: ({ children }) => (
    <div className="my-3 overflow-x-auto rounded-md border border-border">
      <table className="w-full border-collapse text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border-b border-border bg-card/40 px-3 py-2 text-left font-medium">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="border-b border-border/60 px-3 py-2 align-top">{children}</td>
  ),
  code: ({ className, children, ...rest }) => {
    const text = String(children ?? '').replace(/\n$/, '')
    const lang = /language-(\w+)/.exec(className ?? '')?.[1]
    // Inline code: no language class. Render compactly.
    if (!lang) {
      return (
        <code
          className="rounded bg-secondary px-1.5 py-0.5 font-mono text-[0.85em]"
          {...rest}
        >
          {children}
        </code>
      )
    }
    if (lang === 'mermaid') {
      return (
        <Suspense
          fallback={
            <div className="my-4 rounded-md border border-border bg-card/40 p-3 text-xs text-muted-foreground">
              Loading diagram…
            </div>
          }
        >
          <Mermaid chart={text} />
        </Suspense>
      )
    }
    return (
      <pre className="my-3 overflow-x-auto rounded-md border border-border bg-card/40 p-3 text-xs leading-relaxed">
        <code className={`language-${lang} font-mono`}>{text}</code>
      </pre>
    )
  },
}

function MarkdownView({ markdown }: { markdown: string }) {
  return (
    <div className="text-foreground">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
        {markdown}
      </ReactMarkdown>
    </div>
  )
}

// AnswerAudioControl plays the reference-answer audio, or generates it on
// demand (same UX as before this change — extracted here from QuestionDetail
// so AnswerExplanation owns the whole reference-answer surface).
function AnswerAudioControl({
  questionId,
  audioUrl,
}: {
  questionId: string
  audioUrl: string
}) {
  const [url, setUrl] = useState(audioUrl)
  const [loading, setLoading] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  useEffect(() => {
    if (audioUrl) setUrl(audioUrl)
  }, [audioUrl])

  if (url) {
    return <audio src={url} controls preload="metadata" className="h-9 w-full max-w-xs sm:w-auto" />
  }

  const onGenerate = async () => {
    setLoading(true)
    setErrorMsg(null)
    try {
      const generated = await generateQuestionAudio(questionId)
      setUrl(generated)
    } catch (err) {
      setErrorMsg((err as Error)?.message ?? 'Could not generate audio.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <Button variant="outline" size="sm" onClick={onGenerate} disabled={loading}>
        {loading ? (
          <>
            <Loader2 className="size-4 animate-spin" /> Generating…
          </>
        ) : (
          <>
            <Volume2 className="size-4" /> Generate audio
          </>
        )}
      </Button>
      {errorMsg && <span className="text-xs text-red-300">{errorMsg}</span>}
    </div>
  )
}
