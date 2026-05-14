import * as React from 'react'
import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'flex h-9 w-full rounded-md border border-input bg-background/40 px-3 py-1 text-sm',
        'shadow-sm transition-colors placeholder:text-muted-foreground',
        'file:border-0 file:bg-transparent file:text-sm file:font-medium',
        'disabled:cursor-not-allowed disabled:opacity-50',
        'outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:border-ring/60',
        'aria-invalid:border-destructive aria-invalid:focus-visible:ring-destructive',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
