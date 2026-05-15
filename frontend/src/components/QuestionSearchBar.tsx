import { useEffect, useRef, useState } from 'react'
import { Search, X } from 'lucide-react'

import { cn } from '@/lib/utils'

interface QuestionSearchBarProps {
  // The current committed search value (typically read from the URL).
  value: string
  // Called with the debounced user input (300 ms). Empty string == cleared.
  onCommit: (next: string) => void
  placeholder?: string
  className?: string
}

const DEBOUNCE_MS = 300

// Debounced search input. Local state drives the visible value so typing is
// snappy; `onCommit` fires after the user pauses, syncing the URL/query.
export function QuestionSearchBar({
  value,
  onCommit,
  placeholder = 'Search questions — try "how do I scale a service" or "useEffect"',
  className,
}: QuestionSearchBarProps) {
  const [local, setLocal] = useState(value)
  const inputRef = useRef<HTMLInputElement>(null)

  // External resets (e.g. clearFilters or back-button) need to sync down.
  useEffect(() => {
    setLocal(value)
  }, [value])

  // Debounce the commit; cancel pending commit on every keystroke.
  useEffect(() => {
    if (local === value) return
    const id = window.setTimeout(() => onCommit(local), DEBOUNCE_MS)
    return () => window.clearTimeout(id)
  }, [local, value, onCommit])

  const clear = () => {
    setLocal('')
    onCommit('')
    inputRef.current?.focus()
  }

  return (
    <div
      className={cn(
        'group relative flex items-center rounded-xl border border-border/60 bg-card/40',
        'transition-colors focus-within:border-brand-400/60 focus-within:bg-card/60',
        className,
      )}
    >
      <Search className="ml-3 size-4 shrink-0 text-muted-foreground transition-colors group-focus-within:text-brand-300" />
      <input
        ref={inputRef}
        type="search"
        autoComplete="off"
        spellCheck
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') clear()
          if (e.key === 'Enter') onCommit(local)
        }}
        placeholder={placeholder}
        aria-label="Search questions"
        className={cn(
          'h-11 w-full bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground/80',
          '[&::-webkit-search-cancel-button]:hidden [&::-webkit-search-decoration]:hidden',
        )}
      />
      {local && (
        <button
          type="button"
          onClick={clear}
          aria-label="Clear search"
          className="mr-2 rounded-md p-1 text-muted-foreground transition-colors hover:bg-card hover:text-foreground"
        >
          <X className="size-4" />
        </button>
      )}
    </div>
  )
}
