'use client';

import { HelpCircle } from 'lucide-react';
import {
  ReactNode, useEffect, useRef, useState,
} from 'react';

import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';

export function TooltipHelp({
  tooltips,
  trigger,
  asChild,
}: {
  trigger?: ReactNode
  tooltips: string | React.ReactNode
  asChild?: boolean
}) {
  const [open, setOpen] = useState(false);
  const [isHovering, setIsHovering] = useState(false);
  const reopenTimeoutRef = useRef<number | null>(null);

  // When tooltip content (string) changes while hovering, close then reopen to refresh content without requiring re-hover.
  // Guard to strings only to avoid ReactNode identity changes (e.g. <MouseTooltip />) causing flicker.
  useEffect(() => {
    if (!isHovering) return;
    if (typeof tooltips !== 'string') return;
    if (reopenTimeoutRef.current) window.clearTimeout(reopenTimeoutRef.current);
    setOpen(false);
    reopenTimeoutRef.current = window.setTimeout(() => setOpen(true), 0);
  }, [tooltips, isHovering]);

  useEffect(() => () => {
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

  const handleMouseLeave = () => {
    setIsHovering(false);
    setOpen(false);
  };

  const handleMouseEnter = () => {
    setIsHovering(true);
    setOpen(true);
  };

  return (
    <TooltipProvider>
      <Tooltip open={open} onOpenChange={handleOpenChange}>
        <TooltipTrigger asChild={asChild}>
          <span
            className="inline-flex align-middle"
            onMouseEnter={handleMouseEnter}
            onMouseLeave={handleMouseLeave}
          >
            {triggerNode}
          </span>
        </TooltipTrigger>
        <TooltipContent className="p-4 z-100" onMouseLeave={handleMouseLeave} onMouseEnter={handleMouseEnter}>
          {typeof tooltips === 'string' ? <p className="max-w-xs">{tooltips}</p> : tooltips}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
