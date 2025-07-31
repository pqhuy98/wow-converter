import { ExportCharacterRequest } from './server';

export const startupRequests: ExportCharacterRequest[] = [
  {
    character: {
      base: { type: 'wowhead', value: 'https://www.wowhead.com/npc=71865/garrosh-hellscream' },
      size: 'hero',
      attackTag: '2H',
      inGameMovespeed: 270,
      attachItems: {
        1: {
          path: { type: 'wowhead', value: 'https://www.wowhead.com/item=28773/gorehowl' },
        },
      },
      portraitCameraSequenceName: 'Stand',
    },
    outputFileName: 'demo-garrosh',
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
