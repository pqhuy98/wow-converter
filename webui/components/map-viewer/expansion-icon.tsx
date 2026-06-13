import { cn } from '@/lib/utils/css';
import { getWowExpansion } from '@/lib/wow-expansions';

export function ExpansionIcon({
  expansionID,
  className,
}: {
  expansionID: number;
  className?: string;
}) {
  const expansion = getWowExpansion(expansionID);
  return (
    <span
      className={cn('expansion-icon', className)}
      data-expansion={expansionID}
      title={expansion?.name ?? `Expansion ${expansionID}`}
      aria-hidden
    />
  );
}
