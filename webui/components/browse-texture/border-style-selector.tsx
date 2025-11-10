'use client';

import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import type { IconStyle } from '@/lib/models/icon-export.model';

interface BorderStyleSelectorProps {
  value: IconStyle;
  onValueChange: (value: IconStyle) => void;
  disabled?: boolean;
}

export default function BorderStyleSelector({
  value,
  onValueChange,
  disabled = false,
}: BorderStyleSelectorProps) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-sm text-muted-foreground">Border style:</span>
      <Select
        value={value}
        onValueChange={onValueChange}
        disabled={disabled}
      >
        <SelectTrigger className="w-[180px]">
          <SelectValue>
            {value === 'classic-hd-2.0' ? 'Classic HD 2.0' : value === 'reforged-hd' ? 'Reforged HD' : 'Classic SD'}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="classic-hd-2.0">Classic HD 2.0</SelectItem>
          <SelectItem value="reforged-hd">Reforged HD</SelectItem>
          <SelectItem value="classic-sd">Classic SD</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}
