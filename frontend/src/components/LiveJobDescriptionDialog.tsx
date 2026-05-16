import { useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

const MAX_CHARS = 5000

export function LiveJobDescriptionDialog({
  open,
  onOpenChange,
  value,
  onSave,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  value: string
  onSave: (v: string) => void
}) {
  // Local draft so closing without Save doesn't mutate the parent's state.
  const [draft, setDraft] = useState(value)
  useEffect(() => {
    if (open) setDraft(value)
  }, [open, value])

  const trimmed = draft.trim()
  const remaining = MAX_CHARS - draft.length

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Target job description</DialogTitle>
          <DialogDescription>
            Paste the job description you're preparing for. The interviewer will tailor questions
            to its skills and responsibilities — on top of your resume and profile.
          </DialogDescription>
        </DialogHeader>

        <Textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value.slice(0, MAX_CHARS))}
          placeholder="Paste the role's responsibilities, required skills, tech stack…"
          rows={10}
          className="min-h-48"
        />
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>Optional. Stored only for this interview.</span>
          <span>{remaining.toLocaleString()} chars left</span>
        </div>

        <DialogFooter>
          {value && (
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                onSave('')
                onOpenChange(false)
              }}
            >
              Clear
            </Button>
          )}
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="brand"
            onClick={() => {
              onSave(trimmed)
              onOpenChange(false)
            }}
            disabled={!trimmed}
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
