import {
  type Force, type Info, ScriptLanguage, SupportedModes, type Terrain,
} from '../data';

export function defaultInfo(): Info {
  return {
    fileVersion: 33,
    saves: 1,
    editorVersion: 6116,
    gameVersion: { major: 2, minor: 0, patch: 3, build: 22978 },
    map: {
      name: '',
      author: '',
      description: '',
      recommendedPlayers: '',
      playableArea: { width: 64, height: 64 },
      flags: {
        hideMinimapInPreview: false,
        modifyAllyPriorities: false,
        isMeleeMap: true,
        nonDefaultTilesetMapSizeLargeNeverBeenReducedToMedium: true,
        maskedPartiallyVisible: true,
        fixedPlayerSetting: false,
        useCustomForces: false,
        useCustomTechtree: false,
        useCustomAbilities: false,
        useCustomUpgrades: false,
        mapPropertiesMenuOpenedAtLeastOnce: true,
        waterWavesOnCliffShores: true,
        waterWavesOnRollingShores: true,
        useTerrainFog: false,
        tftRequired: false,
        useItemClassificationSystem: true,
        enableWaterTinting: false,
        useAccurateProbabilityForCalculations: false,
        useCustomAbilitySkins: false,
      },
      mainTileType: 'L',
    },
    camera: { bounds: [], complements: [0, 0, 0, 0] },
    gameDataSet: 0,
    prologue: { path: '', text: '', title: '', subtitle: '' },
    loadingScreen: { background: -1, path: '', text: '', title: '', subtitle: '' },
    fog: {
      type: 0,
      startHeight: 3000,
      endHeight: 5000,
      density: 0.5,
      color: [0, 0, 0, 255],
    },
    globalWeather: 0,
    customSoundEnvironment: '',
    customLightEnv: 0,
    water: [255, 255, 255, 255],
    scriptLanguage: ScriptLanguage.JASS,
    supportedModes: SupportedModes.Both,
    gameDataVersion: 1,
    defaultCameraZoom: 1650,
    maxCameraZoom: 1650,
    minCameraZoom: 1650,
    players: [{
      playerNum: 0,
      type: 1,
      race: 1,
      name: 'Player 1',
      startingPos: { x: 0, y: 0, fixed: false },
      allyLowPriorities: 0,
      allyHighPriorities: 0,
      enemyLowPriorities: 0,
      enermyHighPriorities: 0,
    }],
    forces: [{
      flags: {
        allied: false,
        alliedVictory: false,
        shareVision: false,
        shareUnitControl: false,
        shareAdvUnitControl: false,
      },
      players: -1,
      name: '',
    }],
    upgrades: [],
    techBlacklist: [],
    randomUnitTables: [],
    randomItemTables: [],
  };
}

function stripMapExtension(mapName: string): string {
  return mapName.replace(/\.w3x$/i, '');
}

function updateMapBoundsFromTerrain(info: Info, terrain: Terrain): void {
  const tileSize = 128;
  const unplayableLeft = 0;
  const unplayableRight = 0;
  const unplayableBottom = 0;
  const unplayableTop = 0;

  const terrainWidth = terrain.map.width + 1;
  const terrainHeight = terrain.map.height + 1;
  const { offset } = terrain.map;

  info.camera.complements = [
    unplayableLeft,
    unplayableRight,
    unplayableBottom,
    unplayableTop,
  ];
  info.map.playableArea.width = terrainWidth - 1 - unplayableLeft - unplayableRight;
  info.map.playableArea.height = terrainHeight - 1 - unplayableBottom - unplayableTop;

  const leftBottomX = (unplayableLeft + 4) * tileSize + offset.x;
  const leftBottomY = (unplayableBottom + 2) * tileSize + offset.y;
  const rightTopX = (terrainWidth - 1 - unplayableRight - 4) * tileSize + offset.x;
  const rightTopY = (terrainHeight - 1 - unplayableTop - 2) * tileSize + offset.y;

  info.camera.bounds = [
    leftBottomX, leftBottomY,
    rightTopX, rightTopY,
    leftBottomX, rightTopY,
    rightTopX, leftBottomY,
  ];
}

/** Fill war3map.w3i metadata when missing (required by World Editor). */
export function ensureMapInfo(info: Info | undefined, terrain: Terrain, mapSaveName: string): Info {
  const out = info?.fileVersion ? { ...info } : defaultInfo();

  const tileset = terrain.tileset?.trim();
  if (tileset) {
    out.map.mainTileType = tileset.slice(0, 1);
  }
  if (terrain.map.width > 0 && terrain.map.height > 0) {
    updateMapBoundsFromTerrain(out, terrain);
  }
  if (mapSaveName && !out.map.name) {
    out.map.name = stripMapExtension(mapSaveName);
  }
  if (!out.players?.length) {
    out.players = defaultInfo().players;
  }
  if (!out.forces?.length) {
    out.forces = defaultInfo().forces;
  }
  return out;
}
