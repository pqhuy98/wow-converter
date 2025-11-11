'use client';

import { Loader2 } from 'lucide-react';
import { useEffect } from 'react';

interface IconImageProps {
  src: string;
  alt?: string;
  width: number;
  height: number;
  className?: string;
  containerClassName?: string;
  onLoad?: () => void;
  onError?: () => void;
  loading?: 'lazy' | 'eager';
  fallbackUrl?: string;
  isLoading?: boolean;
  showLoadingSpinner?: boolean;
  showCheckerboard?: boolean;
}

/**
 * Reusable icon image component with checkerboard background pattern
 * that adapts to light/dark mode
 */
export function IconImage({
  src,
  alt = '',
  width,
  height,
  className = '',
  containerClassName = '',
  onLoad,
  onError,
  loading = 'lazy',
  fallbackUrl,
  isLoading = false,
  showLoadingSpinner = false,
  showCheckerboard = true,
}: IconImageProps) {
  // Add global checkerboard styles once (persistent, no cleanup needed)
  useEffect(() => {
    const styleId = 'checkerboard-bg-icon-styles';
    if (document.getElementById(styleId)) return;

    const style = document.createElement('style');
    style.id = styleId;
    style.textContent = `
      .checkerboard-bg-icon {
        background-image: linear-gradient(45deg, #d0d0d0 25%, transparent 25%), linear-gradient(-45deg, #d0d0d0 25%, transparent 25%), linear-gradient(45deg, transparent 75%, #d0d0d0 75%), linear-gradient(-45deg, transparent 75%, #d0d0d0 75%);
      }
      .dark .checkerboard-bg-icon {
        background-image: linear-gradient(45deg, #404040 25%, transparent 25%), linear-gradient(-45deg, #404040 25%, transparent 25%), linear-gradient(45deg, transparent 75%, #404040 75%), linear-gradient(-45deg, transparent 75%, #404040 75%);
      }
    `;
    document.head.appendChild(style);
  }, []);

  return (
    <div
      className={`${!isLoading && showCheckerboard ? 'checkerboard-bg-icon' : ''} ${containerClassName}`}
      style={{
        width: `${width}px`,
        height: `${height}px`,
        ...(!isLoading && showCheckerboard ? {
          backgroundSize: '16px 16px',
          backgroundPosition: '0 0, 0 8px, 8px -8px, -8px 0px',
        } : {}),
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius: '0.25rem',
      }}
    >
      {fallbackUrl && (
        <img
          src={fallbackUrl}
          alt=""
          className="absolute inset-0 w-full h-full object-contain rounded blur-sm z-0 opacity-50"
          style={{ imageRendering: 'pixelated' }}
          loading="eager"
        />
      )}
      {showLoadingSpinner && isLoading && (
        <div className="absolute inset-0 flex items-center justify-center z-20">
          <Loader2 className="h-12 w-12 animate-spin text-white" />
        </div>
      )}
      <img
        src={src}
        alt={alt}
        className={`w-auto h-auto max-w-full max-h-full object-contain relative z-10 transition-opacity ${className} ${isLoading ? 'opacity-0' : 'opacity-100'}`}
        loading={loading}
        onLoad={onLoad}
        onError={onError}
      />
    </div>
  );
}
