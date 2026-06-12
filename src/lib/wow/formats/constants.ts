/**
 * Constants for the native WoW reader, ported from wow.export (src/js/constants.js).
 * Cache paths are rooted in the wow-converter project (.cache/wow) instead of
 * the NW.js user-data directory.
 */
import path from 'path';

const DATA_PATH = process.env.WOW_READER_DATA_PATH || path.resolve('.cache', 'wow');

/** Default directory for exported OBJ/PNG/etc. (overridable via WOW_EXPORT_DIR). */
export const EXPORT_PATH = process.env.WOW_EXPORT_DIR || path.resolve('.cache', 'wow-export');

export interface ProductInfo {
  product: string;
  title: string;
  tag: string;
}

export const constants = {
  DATA_PATH,

  // Version stamped into export artifacts (e.g. OBJ headers). Mirrors the
  // wow.export build wow-converter ran against, for byte-identical output.
  VERSION: '0.2.1',

  // User-agent used for HTTP/HTTPs requests.
  USER_AGENT: 'wow.export (1.0.0)',

  // Filter used to filter out WMO LOD files.
  LISTFILE_MODEL_FILTER: /(_\d\d\d_)|(_\d\d\d.wmo$)|(lod\d.wmo$)/,

  GAME: {
    MAP_SIZE: 64,
    MAP_SIZE_SQ: 4096, // MAP_SIZE ^ 2
    MAP_COORD_BASE: 51200 / 3,
    TILE_SIZE: (51200 / 3) / 32,
    MAP_OFFSET: 17066,
  },

  CACHE: {
    DIR: path.join(DATA_PATH, 'casc'), // Cache directory.
    SIZE: path.join(DATA_PATH, 'casc', 'cachesize'), // Cache size.
    INTEGRITY_FILE: path.join(DATA_PATH, 'casc', 'cacheintegrity'), // Cache integrity file.
    SIZE_UPDATE_DELAY: 5000, // Milliseconds to buffer cache size update writes.
    DIR_BUILDS: path.join(DATA_PATH, 'casc', 'builds'), // Build-specific cache directory.
    DIR_INDEXES: path.join(DATA_PATH, 'casc', 'indices'), // Cache for archive indexes.
    DIR_DATA: path.join(DATA_PATH, 'casc', 'data'), // Cache for single data files.
    DIR_DBD: path.join(DATA_PATH, 'casc', 'dbd'), // Cache for DBD files.
    DIR_LISTFILE: path.join(DATA_PATH, 'casc', 'listfile'), // Master listfile cache directory.
    BUILD_MANIFEST: 'manifest.json', // Build-specific manifest file.
    BUILD_LISTFILE: 'listfile', // Build-specific listfile file.
    BUILD_ENCODING: 'encoding', // Build-specific encoding file.
    BUILD_ROOT: 'root', // Build-specific root file.
    LISTFILE_DATA: 'listfile.txt', // Master listfile data file.
    TACT_KEYS: path.join(DATA_PATH, 'tact.json'), // Tact key cache.
  },

  // product: Internal product ID.
  // title: Label as it appears on the Battle.net launcher.
  // tag: Specific version tag.
  PRODUCTS: [
    { product: 'wow', title: 'World of Warcraft', tag: 'Retail' },
    { product: 'wowt', title: 'PTR: World of Warcraft', tag: 'PTR' },
    { product: 'wowxptr', title: 'PTR 2: World of Warcraft', tag: 'PTR 2' },
    { product: 'wow_beta', title: 'Beta: World of Warcraft', tag: 'Beta' },
    { product: 'wow_classic', title: 'World of Warcraft Classic', tag: 'Classic' },
    { product: 'wow_classic_beta', title: 'Beta: World of Warcraft Classic', tag: 'Classic Beta' },
    { product: 'wow_classic_ptr', title: 'PTR: World of Warcraft Classic', tag: 'Classic PTR' },
    { product: 'wow_classic_era', title: 'World of Warcraft Classic Era', tag: 'Classic Era' },
    { product: 'wow_classic_era_ptr', title: 'PTR: World of Warcraft Classic Era', tag: 'Classic Era PTR' },
  ] as ProductInfo[],

  PATCH: {
    REGIONS: ['eu', 'us', 'kr', 'tw', 'cn'], // Valid CDN regions.
    DEFAULT_REGION: 'us', // Region which is selected by default.
    HOST: 'https://%s.version.battle.net/', // Blizzard patch server host.
    SERVER_CONFIG: '/cdns', // CDN config file on patch server.
    VERSION_CONFIG: '/versions', // Versions config file on patch server.
  },

  BUILD: {
    MANIFEST: '.build.info', // File that contains version information in local installs.
    DATA_DIR: 'Data',
  },

  TIME: {
    DAY: 86400000, // Milliseconds in a day.
  },

  MAGIC: {
    M3DT: 0x5444334D, // M3 model magic.
    MD21: 0x3132444D, // M2 model magic.
    MD20: 0x3032444D, // M2 model magic (legacy)
  },
};

export default constants;
