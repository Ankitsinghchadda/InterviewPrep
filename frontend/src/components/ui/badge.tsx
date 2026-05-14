import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center justify-center rounded-full border px-2.5 py-0.5 text-xs font-medium w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 [&>svg]:pointer-events-none gap-1 transition-colors',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-transparent bg-secondary text-secondary-foreground',
        brand:
          'border-transparent bg-brand-500/15 text-brand-200 ring-1 ring-inset ring-brand-400/30',
        outline: 'text-foreground border-border',
        success: 'border-transparent bg-emerald-500/15 text-emerald-300 ring-1 ring-inset ring-emerald-500/30',
        destructive:
          'border-transparent bg-destructive/15 text-red-300 ring-1 ring-inset ring-destructive/30',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {
  asChild?: boolean
}

function Badge({ className, variant, asChild = false, ...props }: BadgeProps) {
  const Comp = asChild ? Slot : 'span'
  return (
    <Comp data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />
  )
}

export { Badge, badgeVariants }
