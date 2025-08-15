import { commonAttachments, otherAttachments } from "@/lib/models/export-character.model"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"


export function AttachmentSelector({
  value,
  onChange,
  usedIds,
}: {
  value: number
  onChange: (id: number) => void
  usedIds: Set<number>
}) {
  const availableCommon = commonAttachments.filter((att) => att.id === value || !usedIds.has(att.id))
  const availableOther = otherAttachments.filter((att) => att.id === value || !usedIds.has(att.id))

  return (
    <Select value={value.toString()} onValueChange={(val) => onChange(Number(val))}>
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="start">
        {availableCommon.map((attachment) => (
          <SelectItem key={attachment.id} value={attachment.id.toString()}>
            {attachment.name} ({attachment.id})
          </SelectItem>
        ))}
        {availableOther.length > 0 && (
          <>
            <div className="px-2 py-1.5 text-xs text-muted-foreground border-t">Other attachments (untested):</div>
            {availableOther.map((attachment) => (
              <SelectItem key={attachment.id} value={attachment.id.toString()}>
                {attachment.name} ({attachment.id})
              </SelectItem>
            ))}
          </>
        )}
      </SelectContent>
    </Select>
  )
}
