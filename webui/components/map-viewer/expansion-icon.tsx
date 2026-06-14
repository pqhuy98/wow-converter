import { cn } from '@/lib/utils/css';
import { WOW_EXPANSIONS } from '@/lib/utils/wow-expansions';

export function ExpansionIcon({
  expansionID,
  className,
}: {
  expansionID: number;
  className?: string;
}) {
  const expansion = WOW_EXPANSIONS.find((e) => e.id === expansionID);
  return (
    <span
      className={cn('expansion-icon', className)}
      data-expansion={expansionID}
      title={expansion?.name ?? `Expansion ${expansionID}`}
      aria-hidden
    />
  );
}
