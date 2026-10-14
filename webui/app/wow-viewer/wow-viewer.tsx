'use client';

import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';

import type { WowConfigStatus } from '@/components/wow-config/wow-config-context';

export default function WowViewer() {
  return <Suspense fallback={<div>Loading…</div>}><Viewer /></Suspense>;
}

function Viewer() {
  const search = useSearchParams();
  const model = search.get('model');
  const product = search.get('product') ?? '';
  const [readyProduct, setReadyProduct] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setError('');
    async function loadProduct() {
      if (product !== 'wow' && product !== 'wow_classic') throw new Error('Unknown WoW product.');
      const response = await fetch('/api/wow-config/status');
      if (!response.ok) throw new Error('Could not read WoW configuration.');
      const status: WowConfigStatus = await response.json();
      if (!status.config) throw new Error('Configure WoW data before opening this model.');
      const loadedProduct = status.cascInfo?.build.Product ?? status.config.product;
      if (!status.cascLoaded || loadedProduct !== product) {
        if (status.cascLoaded) {
          const reset = await fetch('/api/wow-config/reset', { method: 'POST' });
          if (!reset.ok) throw new Error('Could not switch WoW data.');
        }
        const applied = await fetch('/api/wow-config/apply', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ...status.config, product }),
        });
        if (!applied.ok) throw new Error('Could not load the requested WoW data.');
      }
      if (!cancelled) setReadyProduct(product);
    }
    if (product !== '') {
      void loadProduct().catch((err: Error) => {
        if (!cancelled) setError(err.message);
      });
    }
    return () => { cancelled = true; };
  }, [product]);

  if (!model) return <div className="p-8">No model specified. Use /wow-viewer?model=spells/acidliquidbreath.m2</div>;
  if (error) return <div className="p-8">{error}</div>;
  if (product !== '' && readyProduct !== product) return <div className="p-8">Loading {product === 'wow_classic' ? 'Classic' : 'Retail'} WoW data…</div>;
  const params = new URLSearchParams({ model, interactive: '1' });
  const skin = search.get('skin');
  if (skin) params.set('skin', skin);
  return <iframe key={`${model}:${product}`} title={`WoW renderer: ${model}`} src={`/wow-source/index.html?${params.toString()}`} className="fixed inset-0 z-50 h-screen w-screen border-0 bg-[#262626]" />;
}
