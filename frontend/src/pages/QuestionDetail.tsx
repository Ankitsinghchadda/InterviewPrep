import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, BookOpen, Loader2, Mic, Volume2 } from 'lucide-react'

import { useQuestion, useStreamSubmission, useSubmitAnswer } from '@/hooks/queries'
import { generateQuestionAudio } from '@/services/questions'
import type { Difficulty } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Recorder, type RecordingPayload } from '@/components/Recorder'
import { FeedbackCard } from '@/components/FeedbackCard'

const DIFFICULTY_VARIANT: Record<Difficulty, 'success' | 'brand' | 'destructive'> = {
  easy: 'success',
  medium: 'brand',
  hard: 'destructive',
}

export function QuestionDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: question, isLoading, error } = useQuestion(id)

  const [submissionId, setSubmissionId] = useState<string | null>(null)
  const submit = useSubmitAnswer(id || '')
  const stream = useStreamSubmission(submissionId)

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Loading question…
      </div>
    )
  }

  if (error || !question) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Question not found</CardTitle>
          <CardDescription>It may have been removed or you don’t have access.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant="outline">
            <Link to="/questions">
              <ArrowLeft className="size-4" /> Back to questions
            </Link>
          </Button>
        </CardContent>
      </Card>
    )
  }

  const onSubmit = async (payload: RecordingPayload) => {
    try {
      const sub = await submit.mutateAsync(payload)
      setSubmissionId(sub.id)
    } catch (err) {
      console.error('submit failed', err)
    }
  }

  const startOver = () => {
    setSubmissionId(null)
    submit.reset()
  }

  return (
    <article className="space-y-6">
      <div>
        <Link
          to="/questions"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-4" /> All questions
        </Link>
      </div>

      <header className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={DIFFICULTY_VARIANT[question.difficulty] || 'outline'}>
            {question.difficulty}
          </Badge>
          {question.categories.slice(0, 6).map((slug) => (
            <Badge key={slug} variant="outline" className="text-muted-foreground">
              {slug}
            </Badge>
          ))}
        </div>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{question.title}</h1>
        {question.body && <p className="text-muted-foreground">{question.body}</p>}
      </header>

      <Tabs defaultValue="answer">
        <TabsList>
          <TabsTrigger value="answer">
            <BookOpen className="size-4" /> Reference answer
          </TabsTrigger>
          <TabsTrigger value="practice">
            <Mic className="size-4" /> Practice
          </TabsTrigger>
        </TabsList>

        <TabsContent value="answer">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between gap-3 space-y-0">
              <div>
                <CardTitle className="text-base">Reference answer</CardTitle>
                <CardDescription>A solid baseline — your own answer can go deeper.</CardDescription>
              </div>
              <AnswerAudioControl
                questionId={question.id}
                audioUrl={question.answerAudioUrl ?? ''}
              />
            </CardHeader>
            <CardContent>
              <p className="whitespace-pre-wrap text-sm leading-relaxed">{question.answer}</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="practice" className="space-y-5">
          {!submissionId && !submit.isPending && (
            <Card>
              <CardHeader>
                <div className="mb-2 inline-flex size-10 items-center justify-center rounded-lg bg-brand-500/15 text-brand-300 ring-1 ring-inset ring-brand-500/30">
                  <Mic className="size-5" />
                </div>
                <CardTitle>Record your answer</CardTitle>
                <CardDescription>
                  Speak your answer aloud (up to 90 seconds). When you submit, we’ll transcribe it and
                  the AI interviewer will score it with strengths and improvements.
                </CardDescription>
              </CardHeader>
            </Card>
          )}

          {!submissionId && (
            <Recorder onSubmit={onSubmit} busy={submit.isPending} />
          )}

          {submit.isError && (
            <Card className="border-destructive/40">
              <CardHeader>
                <CardTitle className="text-base text-red-300">Upload failed</CardTitle>
                <CardDescription>
                  {(submit.error as Error)?.message || 'Try recording again in a moment.'}
                </CardDescription>
              </CardHeader>
            </Card>
          )}

          {submissionId && (
            <>
              <FeedbackCard
                submission={stream.submission}
                streamingText={stream.reviewText}
                streamingTranscript={stream.transcript}
                streamingStatus={stream.status}
                errorMessage={stream.errorMessage}
              />
              {(stream.status === 'complete' || stream.status === 'failed') && (
                <div className="flex justify-end">
                  <Button variant="outline" onClick={startOver}>
                    Record again
                  </Button>
                </div>
              )}
            </>
          )}
        </TabsContent>
      </Tabs>
    </article>
  )
}

// AnswerAudioControl shows the reference-answer audio player. If the audio
// URL is already on the question, it renders the native player immediately.
// Otherwise it shows a "Generate audio" button that calls the lazy backend
// endpoint (Google TTS → GCS) and swaps in the player when the URL arrives.
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

  // If the parent's audioUrl appears later (e.g., eager-gen finishes), pick it up.
  useEffect(() => {
    if (audioUrl) setUrl(audioUrl)
  }, [audioUrl])

  if (url) {
    return <audio src={url} controls preload="metadata" className="h-9" />
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
