'use client';

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils/css';
import {
  WOW_EXPANSION_ALL,
  type WowExpansion,
} from '@/lib/wow-expansions';

import { ExpansionIcon } from './expansion-icon';

function expansionValue(id: number) {
  return String(id);
}

function expansionIconClass(sizeClass: string) {
  return cn(sizeClass, 'shrink-0');
}

export function ExpansionFilterBar({
  expansions,
  value,
  onChange,
  className,
}: {
  expansions: WowExpansion[];
  value: number;
  onChange: (expansionID: number) => void;
  className?: string;
}) {
  if (expansions.length === 0) return null;

  const isAll = value === WOW_EXPANSION_ALL;

  return (
    <Select
      value={expansionValue(value)}
      onValueChange={(v) => onChange(Number(v))}
    >
      <SelectTrigger
        className={cn(
          'h-10 shrink-0 gap-1 px-2 py-0 w-[5rem]',
          '[&>span:first-child]:flex [&>span:first-child]:items-center [&>span:first-child]:justify-center',
          '[&>span:first-child]:leading-none [&>span:first-child]:line-clamp-none',
          className,
        )}
        aria-label="Filter by expansion"
      >
        <SelectValue>
          {isAll ? (
            <span className="text-sm text-muted-foreground">All</span>
          ) : (
            <ExpansionIcon expansionID={value} className={expansionIconClass('!w-9 !h-5')} />
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent align="end">
        <SelectItem value={expansionValue(WOW_EXPANSION_ALL)}>All</SelectItem>
        {expansions.map((exp) => (
          <SelectItem key={exp.id} value={expansionValue(exp.id)}>
            <span className="flex items-center gap-2">
              <ExpansionIcon expansionID={exp.id} className={expansionIconClass('!size-4')} />
              {exp.name}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
