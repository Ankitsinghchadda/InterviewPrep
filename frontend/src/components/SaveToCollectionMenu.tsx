import { useMemo, useState } from 'react'
import { Bookmark, BookmarkCheck, Check, ChevronDown, Loader2, Plus } from 'lucide-react'

import {
  useAddToCollection,
  useCollections,
  useCollectionsForQuestion,
  useCreateCollection,
  useRemoveFromCollection,
} from '@/hooks/queries'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

interface SaveToCollectionMenuProps {
  questionId: string
  /** Visual size — "lg" used on QuestionDetail header; "sm" on cards. */
  size?: 'sm' | 'lg'
}

// SaveToCollectionMenu is a bookmark button + dropdown that toggles a
// question's membership in the user's collections. Single click on the icon
// toggles the default Saved collection; the dropdown reveals all collections
// for multi-add and offers "Create new...".
export function SaveToCollectionMenu({ questionId, size = 'lg' }: SaveToCollectionMenuProps) {
  const { data: collections = [], isLoading: collectionsLoading } = useCollections()
  const { data: memberIds = [], isLoading: memberLoading } = useCollectionsForQuestion(questionId)
  const add = useAddToCollection()
  const remove = useRemoveFromCollection()

  const [createOpen, setCreateOpen] = useState(false)

  const defaultCollection = useMemo(
    () => collections.find((c) => c.isDefault) ?? null,
    [collections],
  )
  const isSavedDefault = defaultCollection
    ? memberIds.includes(defaultCollection.id)
    : false
  const inAnyCollection = memberIds.length > 0

  const toggleDefault = () => {
    if (!defaultCollection) return
    if (isSavedDefault) {
      remove.mutate({ collectionId: defaultCollection.id, questionId })
    } else {
      add.mutate({ collectionId: defaultCollection.id, questionId })
    }
  }

  const toggleMembership = (collectionId: string) => {
    if (memberIds.includes(collectionId)) {
      remove.mutate({ collectionId, questionId })
    } else {
      add.mutate({ collectionId, questionId })
    }
  }

  const Icon = inAnyCollection ? BookmarkCheck : Bookmark
  const iconClass = inAnyCollection ? 'text-brand-300' : 'text-muted-foreground'
  const triggerSize = size === 'sm' ? 'size-8' : 'size-9'
  const iconSize = size === 'sm' ? 'size-4' : 'size-5'

  return (
    <>
      <div className="inline-flex shrink-0 items-stretch rounded-md border border-border/60 bg-background/40">
        <button
          type="button"
          onClick={toggleDefault}
          disabled={collectionsLoading || memberLoading || add.isPending || remove.isPending}
          aria-pressed={isSavedDefault}
          title={isSavedDefault ? 'Remove from Saved' : 'Save'}
          className={cn(
            'inline-flex shrink-0 items-center justify-center rounded-l-md px-2 transition-colors hover:bg-accent/40 disabled:opacity-50',
            triggerSize,
          )}
        >
          <Icon className={cn(iconSize, iconClass)} />
        </button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              title="Save to collection…"
              aria-label="Save to collection"
              className={cn(
                'inline-flex shrink-0 items-center justify-center gap-1 whitespace-nowrap rounded-r-md border-l border-border/60 px-2 text-xs text-muted-foreground transition-colors hover:bg-accent/40',
                size === 'sm' ? 'h-8' : 'h-9',
              )}
            >
              <span className="hidden sm:inline">Save to…</span>
              <ChevronDown className="size-3.5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-60">
            <DropdownMenuLabel>Save to collection</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {collectionsLoading ? (
              <div className="px-2 py-3 text-center text-xs text-muted-foreground">
                <Loader2 className="mx-auto size-4 animate-spin" />
              </div>
            ) : collections.length === 0 ? (
              <div className="px-2 py-3 text-center text-xs text-muted-foreground">
                No collections yet.
              </div>
            ) : (
              collections.map((c) => {
                const checked = memberIds.includes(c.id)
                return (
                  <DropdownMenuItem
                    key={c.id}
                    onSelect={(e) => {
                      e.preventDefault()
                      toggleMembership(c.id)
                    }}
                  >
                    <span
                      className={cn(
                        'grid size-4 place-items-center rounded-sm border border-border/60',
                        checked && 'border-brand-400 bg-brand-500/30 text-brand-100',
                      )}
                    >
                      {checked && <Check className="size-3" />}
                    </span>
                    <span className="flex-1 truncate">{c.name}</span>
                    <span className="text-xs text-muted-foreground">{c.questionCount}</span>
                  </DropdownMenuItem>
                )
              })
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              Create new collection…
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <CreateCollectionDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        questionIdToAdd={questionId}
      />
    </>
  )
}

// CreateCollectionDialog is a small inline form. On success it adds the
// originating question to the newly-created collection so the user doesn't
// have to manually re-tick it.
export function CreateCollectionDialog({
  open,
  onOpenChange,
  questionIdToAdd,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  questionIdToAdd?: string
}) {
  const [name, setName] = useState('')
  const create = useCreateCollection()
  const add = useAddToCollection()
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Name is required.')
      return
    }
    try {
      const c = await create.mutateAsync({ name: trimmed })
      if (questionIdToAdd) {
        await add.mutateAsync({ collectionId: c.id, questionId: questionIdToAdd })
      }
      setName('')
      onOpenChange(false)
    } catch (err) {
      setError((err as Error).message || 'Failed to create collection.')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New collection</DialogTitle>
          <DialogDescription>
            Group questions into a personal list — like "Behavioral" or "FAANG prep".
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="collection-name">Name</Label>
            <Input
              id="collection-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Behavioral"
              maxLength={80}
              autoFocus
            />
          </div>
          {error && <p className="text-sm text-red-300">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? <Loader2 className="size-4 animate-spin" /> : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
