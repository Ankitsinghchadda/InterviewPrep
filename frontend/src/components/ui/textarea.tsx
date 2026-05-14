import * as React from 'react'
import { cn } from '@/lib/utils'

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        'flex min-h-20 w-full rounded-md border border-input bg-background/40 px-3 py-2 text-sm',
        'shadow-sm transition-colors placeholder:text-muted-foreground',
        'disabled:cursor-not-allowed disabled:opacity-50',
        'outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:border-ring/60',
        'aria-invalid:border-destructive aria-invalid:focus-visible:ring-destructive',
        'resize-y',
        className,
      )}
      {...props}
    />
  )
}

export { Textarea }
