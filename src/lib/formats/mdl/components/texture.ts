export interface Texture {
  id: number
  replaceableId?: number;
  image: string;
  wrapWidth: boolean;
  wrapHeight: boolean;
  wowData: {
    type: number // 0 is default, >=1 is replaceable id
    pngPath: string
  }
}

export function texturesToString(textures: Texture[]) {
  if (textures.length === 0) return '';
  return `Textures ${textures.length} {
    ${textures.map((texture) => `
      Bitmap {
        Image "${texture.image}",
        ${texture.replaceableId ? `ReplaceableId ${texture.replaceableId},` : ''}
        ${texture.wrapWidth ? 'WrapWidth,' : ''}
        ${texture.wrapHeight ? 'WrapHeight,' : ''}
      }`).join('\n')}
  }`;
}
