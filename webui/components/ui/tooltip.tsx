'use client';

import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import * as React from 'react';

import { cn } from '@/lib/utils/css';

const TooltipProvider = React.forwardRef<React.ElementRef<typeof TooltipPrimitive.Provider>, React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Provider>>(({ children, ...props }, _ref) => (
  <TooltipPrimitive.Provider delayDuration={0} {...props}>
    {children}
  </TooltipPrimitive.Provider>
));

TooltipProvider.displayName = TooltipPrimitive.Provider.displayName;

const Tooltip = TooltipPrimitive.Root;

const TooltipTrigger = TooltipPrimitive.Trigger;

const TooltipContent = React.forwardRef<
  React.ElementRef<typeof TooltipPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({
  className, sideOffset = 4, style, ...props
}, ref) => {
  // Radix UI tooltips are portaled to body, but we need to ensure the portal container has proper z-index
  React.useEffect(() => {
    const updatePortalZIndex = () => {
      // Find all Radix portal containers and set their z-index
      const portals = document.querySelectorAll('[data-radix-portal]');
      portals.forEach((portal) => {
        (portal as HTMLElement).style.zIndex = '99999';
      });
      // Also set z-index on tooltip content elements directly
      const tooltipContents = document.querySelectorAll('[data-radix-tooltip-content]');
      tooltipContents.forEach((content) => {
        (content as HTMLElement).style.zIndex = '99999';
      });
    };
    updatePortalZIndex();
    // Watch for dynamically added portals and tooltips
    const observer = new MutationObserver(updatePortalZIndex);
    observer.observe(document.body, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, []);

  return (
    <TooltipPrimitive.Content
      ref={ref}
      sideOffset={sideOffset}
      className={cn(
        'z-[99999] overflow-hidden rounded-md border bg-popover text-popover-foreground px-3 py-1.5 text-sm text-left shadow-md inline-block max-w-[300px] whitespace-normal animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2',
        className,
      )}
      style={{ zIndex: 99999, position: 'relative', ...style }}
      {...props}
    />
  );
});
TooltipContent.displayName = TooltipPrimitive.Content.displayName;

export {
  Tooltip, TooltipTrigger, TooltipContent, TooltipProvider,
};
