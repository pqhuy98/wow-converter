import { expect, test } from 'bun:test';

import { ItemMetata, itemModelTextureFiles, itemReplaceableTextures } from './item-model';

test('item model texture sets stay paired with their model component', () => {
  const itemData: Pick<ItemMetata, 'modelFiles' | 'modelTextureFiles'> = {
    modelFiles: [
      { fileDataId: 4994107, componentId: 0 },
      { fileDataId: 4994072, componentId: 1 },
    ],
    modelTextureFiles: [
      [
        { fileDataId: 5153458, componentId: 2 },
        { fileDataId: 5154025, componentId: 3 },
      ],
      [
        { fileDataId: 5153434, componentId: 2 },
        { fileDataId: 5154043, componentId: 3 },
      ],
    ],
  };

  const firstTextures = itemModelTextureFiles(itemData, 0);
  const secondTextures = itemModelTextureFiles(itemData, 1);
  expect(itemReplaceableTextures(firstTextures)).toEqual({ 2: 5153458, 3: 5154025 });
  expect(itemReplaceableTextures(secondTextures)).toEqual({ 2: 5153434, 3: 5154043 });
});
