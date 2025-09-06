import { Fragment } from 'react';

import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import {
  commonAttachments, hipAttachments, otherAttachments, sheathAttachments,
} from '@/lib/models/export-character.model';

export function AttachmentSelector({
  value,
  onChange,
  usedIds,
}: {
  value: number
  onChange: (id: number) => void
  usedIds: Set<number>
}) {
  const attachments: [typeof commonAttachments, string][] = [
    [commonAttachments, 'Weapon'],
    [sheathAttachments, 'Weapon in Sheath'],
    [hipAttachments, 'Weapon in Hip'],
    [otherAttachments, 'Other'],
  ];

  return (
    <Select value={value.toString()} onValueChange={(val) => onChange(Number(val))}>
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="start">
        {attachments.map(([attachments, sectionName]) => (
          <Fragment key={sectionName}>
            <div className="px-2 py-1.5 text-xs text-muted-foreground border-t">{sectionName}:</div>
            {attachments.map((attachment) => (
              <SelectItem key={attachment.id} value={attachment.id.toString()} disabled={usedIds.has(attachment.id)}>
                {attachment.name} ({attachment.id})
              </SelectItem>
            ))}
          </Fragment>
        ))}
      </SelectContent>
    </Select>
  );
}
