import { ExportCharacterRequest } from './export-character';

export const startupRequests: ExportCharacterRequest[] = [
  {
    character: {
      base: { type: 'wowhead', value: 'https://www.wowhead.com/wotlk/npc=36597/the-lich-king' },
      size: 'hero',
      attackTag: '2H',
      inGameMovespeed: 270,
      attachItems: {
        1: {
          path: { type: 'wowhead', value: 'https://www.wowhead.com/classic/item=231885/frostmourne' },
        },
      },
      portraitCameraSequenceName: 'Stand',
    },
    optimization: {
      sortSequences: true,
      removeUnusedVertices: true,
      removeUnusedNodes: true,
      removeUnusedMaterialsTextures: true,
    },
    outputFileName: 'demo-lichking',
    format: 'mdx',
  },
  // {
  //   character: {
  //     base: { type: 'wowhead', value: 'https://www.wowhead.com/wotlk/npc=37119/highlord-tirion-fordring' },
  //     size: 'hero',
  //     attackTag: '2H',
  //     inGameMovespeed: 270,
  //     attachItems: {
  //       1: {
  //         path: { type: 'wowhead', value: 'https://www.wowhead.com/item=120978/ashbringer' },
  //       },
  //     },
  //     portraitCameraSequenceName: 'Stand',
  //   },
  //   outputFileName: 'demo-tirion',
  //   format: 'mdx',
  // },
];
