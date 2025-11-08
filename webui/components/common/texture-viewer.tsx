'use client';

import { useEffect, useRef, useState } from 'react';

interface TextureViewerProps {
  texturePath?: string;
  onLoad?: () => void;
  onError?: () => void;
}

export default function TextureViewer({
  texturePath,
  onLoad,
  onError,
}: TextureViewerProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const imgRef = useRef<HTMLImageElement | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (texturePath) {
      setIsLoading(true);
    }
  }, [texturePath]);

  if (!texturePath) {
    return (
      <div className="w-full h-full flex items-center justify-center bg-secondary text-muted-foreground">
        <p>Select a texture to view</p>
      </div>
    );
  }

  const encodedPath = encodeURIComponent(texturePath);
  const imageUrl = `/api/texture/png/${encodedPath}`;

  return (
    <div
      ref={containerRef}
      className="w-full h-full flex items-center justify-center bg-secondary overflow-hidden relative"
    >
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center z-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </div>
      )}
      <img
        ref={imgRef}
        src={imageUrl}
        alt={texturePath}
        style={{
          maxWidth: '100%',
          maxHeight: '100%',
          width: 'auto',
          height: 'auto',
          objectFit: 'contain',
        }}
        className="select-none"
        onLoad={() => {
          setIsLoading(false);
          onLoad?.();
        }}
        onError={() => {
          setIsLoading(false);
          onError?.();
        }}
      />
    </div>
  );
}
