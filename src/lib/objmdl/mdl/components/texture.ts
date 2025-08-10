export interface Texture {
  id: number
  image: string;
  wrapWidth: boolean;
  wrapHeight: boolean;
}

export function texturesToString(textures: Texture[]) {
  if (textures.length === 0) return '';
  return `Textures ${textures.length} {
    ${textures.map((texture) => `
      Bitmap {
        Image "${texture.image}",
        ${texture.wrapWidth ? 'WrapWidth,' : ''}
        ${texture.wrapHeight ? 'WrapHeight,' : ''}
      }`).join('\n')}
  }`;
}
