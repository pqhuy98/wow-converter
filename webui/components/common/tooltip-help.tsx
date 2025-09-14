'use client';

import { HelpCircle } from 'lucide-react';
import * as React from 'react';

import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';

export function TooltipHelp({
  tooltips,
  trigger,
}: {
  trigger?: React.ReactNode
  tooltips: string | React.ReactNode
  asChild?: boolean
}) {
  const [open, setOpen] = React.useState(false);
  const [isHovering, setIsHovering] = React.useState(false);
  const reopenTimeoutRef = React.useRef<number | null>(null);

  // When tooltip content changes while hovering, close then reopen to refresh content without requiring re-hover
  React.useEffect(() => {
    if (!isHovering) return;
    if (reopenTimeoutRef.current) window.clearTimeout(reopenTimeoutRef.current);
    setOpen(false);
    reopenTimeoutRef.current = window.setTimeout(() => setOpen(true), 0);
  }, [tooltips, isHovering]);

  React.useEffect(() => () => {
    if (reopenTimeoutRef.current) window.clearTimeout(reopenTimeoutRef.current);
  }, []);

  const handleOpenChange = (nextOpen: boolean) => {
    // If Radix tries to close while still hovering, immediately reopen
    if (!nextOpen && isHovering) {
      // Reopen on next frame to avoid fighting Radix internal state
      requestAnimationFrame(() => setOpen(true));
      return;
    }
    setOpen(nextOpen);
  };

  const triggerNode = trigger || <HelpCircle className="h-4 w-4 text-muted-foreground" />;

  return (
    <TooltipProvider>
      <Tooltip open={open} onOpenChange={handleOpenChange}>
        <TooltipTrigger asChild>
          <span
            className="inline-flex align-middle"
            onMouseEnter={() => { setIsHovering(true); setOpen(true); }}
            onMouseLeave={() => { setIsHovering(false); setOpen(false); }}
          >
            {triggerNode}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          {typeof tooltips === 'string' ? <p className="max-w-xs">{tooltips}</p> : tooltips}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
