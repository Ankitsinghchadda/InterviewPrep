// Razorpay Checkout.js is loaded on demand from their CDN so we don't
// add a vendor script to every page. The first call to ensureCheckout()
// injects the <script> tag and resolves once window.Razorpay exists.

const SCRIPT_URL = 'https://checkout.razorpay.com/v1/checkout.js'

declare global {
  interface Window {
    Razorpay?: new (options: RazorpayOptions) => { open(): void }
  }
}

export interface RazorpayOptions {
  key: string
  subscription_id: string
  name?: string
  description?: string
  image?: string
  prefill?: {
    name?: string
    email?: string
    contact?: string
  }
  theme?: {
    color?: string
  }
  handler?: (response: {
    razorpay_payment_id: string
    razorpay_subscription_id: string
    razorpay_signature: string
  }) => void
  modal?: {
    ondismiss?: () => void
  }
}

let loaderPromise: Promise<void> | null = null

export function ensureCheckout(): Promise<void> {
  if (typeof window === 'undefined') return Promise.reject(new Error('no window'))
  if (window.Razorpay) return Promise.resolve()
  if (loaderPromise) return loaderPromise

  loaderPromise = new Promise<void>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${SCRIPT_URL}"]`)
    if (existing && window.Razorpay) {
      resolve()
      return
    }
    const s = document.createElement('script')
    s.src = SCRIPT_URL
    s.async = true
    s.onload = () => {
      if (window.Razorpay) resolve()
      else reject(new Error('Razorpay checkout failed to initialize'))
    }
    s.onerror = () => reject(new Error('Failed to load Razorpay checkout script'))
    document.head.appendChild(s)
  })
  return loaderPromise
}

// openCheckout loads the script (idempotent) then opens the modal.
// Caller's `handler` fires on a successful payment; `onDismiss` fires when
// the user closes the modal without paying. The server still relies on
// the webhook for the source-of-truth upgrade — the handler is just for
// instant UX feedback.
export async function openCheckout(options: RazorpayOptions): Promise<void> {
  await ensureCheckout()
  if (!window.Razorpay) throw new Error('Razorpay not available')
  const rzp = new window.Razorpay(options)
  rzp.open()
}
