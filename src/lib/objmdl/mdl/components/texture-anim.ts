import { QuaternionRotation, Vector3 } from '@/lib/math/common';

import { Animation, animationToString } from './animation';

export interface TextureAnim {
  id: number
  translation?: Animation<Vector3>;
  scaling?: Animation<Vector3>;
  rotation?: Animation<QuaternionRotation>;
}

export function textureAnimsToString(textureAnims: TextureAnim[]): string {
  if (textureAnims.length === 0) return '';
  return `TextureAnims ${textureAnims.length} {
    ${textureAnims.map((texAnim) => `TVertexAnim {
      ${animationToString('Translation', texAnim.translation)}
      ${animationToString('Rotation', texAnim.rotation)}
      ${animationToString('Scaling', texAnim.scaling)}
    }`).join('\n')}
  }`;
}
