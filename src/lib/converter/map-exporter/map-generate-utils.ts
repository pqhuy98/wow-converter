import chalk from 'chalk';

import { distancePerTile, maxGameHeightDiff } from '@/lib/constants';
import { isWowUnit, WowAdt, WowObject } from '@/lib/converter/common/models';
import type { WowObjectManager } from '@/lib/converter/common/wow-object-manager';
import { MapExportConfig, MapExporter } from '@/lib/converter/map-exporter/map-exporter';
import { computeRecommendedTerrainClampPercent } from '@/lib/converter/map-exporter/wc3-converter';
import { Vector3 } from '@/lib/math/common';
import { V3 } from '@/lib/math/vector';

export function computeCreatureExportSteps(uniqueCreatureCount: number): number {
  return uniqueCreatureCount > 0 ? uniqueCreatureCount : 0;
}

export function countUniqueUnitExportsFromManager(wowObjectManager: WowObjectManager): number {
  const displayIds = new Set<number>();
  let count = 0;
  wowObjectManager.iterateObjects((obj) => {
    if (!isWowUnit(obj)) return;
    const displayId = obj.creature.model.CreatureDisplayID;
    if (!displayId || displayIds.has(displayId)) return;
    displayIds.add(displayId);
    count++;
  });
  return count;
}

export function autoChooseClampPercent(
  mapExporter: MapExporter,
  mapExportConfig: MapExportConfig,
  unitScale: number,
): void {
  const unitPos: Vector3[] = [];
  mapExporter.wowObjectManager.iterateObjects((obj, abs) => {
    if (!isWowUnit(obj)) return;
    unitPos.push(abs.position);
  });
  if (unitPos.length === 0) {
    console.log(
      'No units found, cannot auto choose clamp percent. Defaulting to',
      mapExportConfig.terrain.clampPercent.lower,
      mapExportConfig.terrain.clampPercent.upper,
    );
    return;
  }
  unitPos.sort((a, b) => a[2] - b[2]);
  const { ratio, min, max } = computeRecommendedTerrainClampPercent(mapExporter.wowObjectManager.roots);
  let clampDiff = ratio * unitScale;

  const size = V3.sub(max, min);
  const ratioZ = maxGameHeightDiff / (size[2] * clampDiff);
  const width = size[0] * ratioZ / distancePerTile;
  const height = size[1] * ratioZ / distancePerTile;

  const w4 = Math.ceil(width / 4) * 4;
  const h4 = Math.ceil(height / 4) * 4;
  clampDiff *= Math.max(1, w4 / 480, h4 / 480);

  const unitPosRatio = unitPos.map((pos) => (pos[2] - min[2]) / (max[2] - min[2]));

  let bestLowerPercent = 0;
  let bestUpperPercent = ratio;
  let maxCount = 0;
  const lower = mapExportConfig.terrain.clampPercent.lower;
  const upper = mapExportConfig.terrain.clampPercent.upper;
  if (upper - lower <= clampDiff) {
    console.log('Map terrain clamp is already within the recommended range, skipping auto choose.');
    return;
  }

  for (let lowerPercent = lower; lowerPercent <= upper - clampDiff; lowerPercent += 0.01) {
    const upperPercent = lowerPercent + clampDiff;
    const count = unitPosRatio.filter((r) => r >= lowerPercent && r <= upperPercent).length;
    if (count > maxCount) {
      maxCount = count;
      bestLowerPercent = lowerPercent;
      bestUpperPercent = upperPercent;
    }
  }
  mapExportConfig.terrain.clampPercent.lower = bestLowerPercent;
  mapExportConfig.terrain.clampPercent.upper = bestUpperPercent;
  const leftOutBelow = unitPosRatio.filter((r) => r < bestLowerPercent).length;
  const leftOutAbove = unitPosRatio.filter((r) => r > bestUpperPercent).length;
  const leftOut = leftOutBelow + leftOutAbove;
  const remaining = unitPosRatio.length - leftOut;
  console.log(`Chosen clamp percent: ${bestLowerPercent} - ${bestUpperPercent} (${remaining} units remaining)`);
  console.log(`Left out units: ${leftOut} (${leftOutBelow} below, ${leftOutAbove} above)`);
}

export function pruneDepth(mapExporter: MapExporter, depth: number): void {
  const wowObjectManager = mapExporter.wowObjectManager;
  if (!wowObjectManager) return;

  if (depth >= 3) return;

  const nextRoots: WowObject[] = [];
  const nextObjects = new Map<string, WowObject>();
  const nextDoodads: WowObject[] = [];
  const nextTerrains: WowAdt[] = [];

  const visit = (obj: WowObject, hasWmoAncestor: boolean): WowObject | null => {
    const currentHasWmoAncestor = hasWmoAncestor || obj.type === 'wmo';

    let keep = true;
    if (depth === 1) {
      keep = obj.type === 'adt' || obj.type === 'unit';
    } else if (depth === 2) {
      if (obj.type === 'm2' || obj.type === 'gobj') {
        keep = !currentHasWmoAncestor;
      }
    }

    if (!keep) return null;

    const clone: WowObject = { ...obj, children: [] };
    nextObjects.set(clone.id, clone);

    if (clone.type === 'adt') {
      nextTerrains.push(clone as WowAdt);
    } else if (clone.type !== 'unit') {
      nextDoodads.push(clone);
    }

    for (const child of obj.children) {
      const prunedChild = visit(child, currentHasWmoAncestor);
      if (prunedChild) clone.children.push(prunedChild);
    }

    return clone;
  };

  for (const root of wowObjectManager.roots) {
    const newRoot = visit(root, false);
    if (newRoot) nextRoots.push(newRoot);
  }

  if (depth === 1) {
    const hasAdtRoot = nextRoots.some((root) => root.type === 'adt');
    wowObjectManager.roots = hasAdtRoot
      ? nextRoots.filter((root) => root.type === 'adt')
      : nextRoots;
  } else {
    wowObjectManager.roots = nextRoots;
  }

  wowObjectManager.objects = nextObjects;
  wowObjectManager.doodads = nextDoodads;
  if (nextTerrains.length > 0) {
    wowObjectManager.terrains = nextTerrains;
  }
}

export function logMapGeneratePhase(label: string): void {
  console.log(chalk.cyan(`[map-generate] ${label}`));
}
