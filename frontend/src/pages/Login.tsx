import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Sparkles } from 'lucide-react'

import { useAuth } from '@/auth/AuthContext'
import { googleLoginURL } from '@/services/auth'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const ERROR_MESSAGES: Record<string, string> = {
  invalid_state: 'Login session expired. Please try again.',
  exchange_failed: "We couldn't verify your Google account. Please try again.",
  session_failed: 'Sign-in succeeded but we failed to create a session. Try again.',
  missing_params: 'Google returned an incomplete response. Please try again.',
  access_denied: 'You declined the Google sign-in. No worries.',
}

export function Login() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { status } = useAuth()

  const redirect = params.get('redirect') ?? '/dashboard'
  const errorCode = params.get('error')
  const errorMsg = errorCode ? ERROR_MESSAGES[errorCode] ?? 'Sign-in failed. Please try again.' : null

  useEffect(() => {
    if (status === 'authenticated') {
      navigate(redirect, { replace: true })
    }
  }, [status, redirect, navigate])

  return (
    <div className="mx-auto flex min-h-[60vh] max-w-md items-center px-4 py-12">
      <Card className="w-full border-border/70">
        <CardHeader className="items-center text-center">
          <span className="mb-3 grid size-10 place-items-center rounded-lg bg-gradient-to-br from-brand-400 to-brand-700 text-white shadow-md shadow-brand-700/30">
            <Sparkles className="size-5" />
          </span>
          <CardTitle className="text-2xl">Sign in to InterviewPrep</CardTitle>
          <CardDescription>Use your Google account to continue.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-4">
          {errorMsg && (
            <div
              role="alert"
              className="w-full rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-red-300"
            >
              {errorMsg}
            </div>
          )}
          <a
            href={googleLoginURL(redirect)}
            className="inline-flex w-full items-center justify-center gap-3 rounded-md bg-white px-5 py-2.5 font-medium text-zinc-900 shadow-sm transition hover:bg-zinc-100"
          >
            <GoogleIcon />
            <span>Continue with Google</span>
          </a>
          <p className="text-center text-xs text-muted-foreground">
            By signing in you agree to our terms. We won’t spam you.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

function GoogleIcon() {
  return (
    <svg className="size-5" viewBox="0 0 48 48" aria-hidden>
      <path
        fill="#FFC107"
        d="M43.6 20.5H42V20H24v8h11.3c-1.6 4.6-6 8-11.3 8-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.9 1.2 8 3.1l5.7-5.7C34 6.1 29.3 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.4-.4-3.5z"
      />
      <path
        fill="#FF3D00"
        d="M6.3 14.7l6.6 4.8C14.7 16 19 13 24 13c3.1 0 5.9 1.2 8 3.1l5.7-5.7C34 6.1 29.3 4 24 4 16.3 4 9.7 8.3 6.3 14.7z"
      />
      <path
        fill="#4CAF50"
        d="M24 44c5.2 0 9.9-2 13.4-5.2l-6.2-5.2c-2 1.4-4.5 2.2-7.2 2.2-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.5 39.6 16.2 44 24 44z"
      />
      <path
        fill="#1976D2"
        d="M43.6 20.5H42V20H24v8h11.3c-.8 2.2-2.2 4.1-4.1 5.5l6.2 5.2C41.8 35.6 44 30.2 44 24c0-1.3-.1-2.4-.4-3.5z"
      />
    </svg>
  )
}
