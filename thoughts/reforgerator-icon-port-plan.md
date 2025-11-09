# Port Reforgerator Icon Maker - Two Phase Workflow

## Overview
Port the core icon conversion functionality from Reforgerator (Python/PIL) to Node.js (TypeScript/sharp) in wow-converter. The workflow is two-phase:
1. **Phase 1 - Generate PNG icons**: Process input PNGs with frames/styles/extras → export processed PNGs to wow.export asset dir for UI review
2. **Phase 2 - Convert to BLP**: After user review, client makes API calls to convert approved PNGs to BLP → save to exported-assets

## Key Components

### 1. Frame Assets Management
- **Location**: `wow-converter/resources/icon-frames/`
- **Action**: Copy frame PNG assets from `Reforgerator/data/frames/` directory structure
- **Structure**: Maintain `{size}/{style}/{frame}.png` hierarchy (e.g., `64x64/ClassicSD/BTN.png`)
- **Frames**: BTN, DISBTN, PAS, DISPAS, ATC, DISATC, plus extras (Blackframe_big, Heroframe)
- **Styles**: ClassicSD, ReforgedHD, ClassicHD2.0
- **Sizes**: 64x64, 128x128, 256x256

### 2. Core PNG Icon Generation (Phase 1)
- **Location**: `wow-converter/src/lib/formats/icon/convert-png-to-icon.ts`
- **Functions**:
  ```typescript
  // Process PNG with frames/styles/extras → return PNG buffer
  export async function convertPngToIcon(
    inputPng: string | Buffer,
    options: IconConversionOptions
  ): Promise<Buffer>
  
  // Generate and save PNG icon to wow.export asset directory
  export async function generateIconPng(
    inputPng: string | Buffer,
    outputPath: string,  // Relative to wow.export asset dir
    options: IconConversionOptions
  ): Promise<void>
  ```

### 3. Image Processing Pipeline
- **Location**: `wow-converter/src/lib/formats/icon/icon-processor.ts`
- **Steps**: 
  1. Load input PNG using `sharp`
  2. Apply crop if enabled (10% symmetric crop)
  3. Resize to target size (or keep original)
  4. Load and composite frame image
  5. Apply black frame overlay if enabled
  6. Apply hero frame overlay if enabled
  7. Apply desaturation/contrast for disabled frames (HD style)
  8. Handle alpha channel (remove colors from transparent pixels or clear alpha)
  9. Export as PNG buffer (ready for saving to wow.export asset dir)
- **All operations work on PNG buffers** - no BLP until phase 2

### 4. Frame Compositing Logic
- **Location**: `wow-converter/src/lib/formats/icon/icon-processor.ts`
- **Port from**: `Reforgerator/src/converter.py::apply_frame()`
- **Key operations**:
  - Load frame PNG from assets directory
  - Resize frame to match canvas size
  - Use alpha compositing (sharp's `composite()`)
  - Handle custom frame positioning/sizing (for custom frames)
  - Cache loaded frames in memory

### 5. BLP Conversion API (Phase 2)
- **Location**: `wow-converter/src/server/controllers/convert-icon-to-blp.ts`
- **Endpoint**: `POST /api/convert-icon-to-blp`
- **Body**: `{ pngPath: string, outputPath: string }`
- **Process**: Read PNG from wow.export asset dir → convert using existing `pngsToBlps()` from `formats/blp/blp.ts` → save to exported-assets
- **Note**: This is phase 2 - only called after user reviews PNG icons in UI

### 6. Utility Functions
- **Location**: `wow-converter/src/lib/formats/icon/icon-utils.ts`
- **Functions**:
  - `resolveFramePath(size, style, frame)`: Resolve path to frame PNG asset in resources/icon-frames/
  - `optimalCropMargin(dim, cropPercent)`: Calculate crop margins
  - `removeColorsFromAlphaPixels(image)`: Clear RGB where alpha=0
  - `srgbToLinear()` / `linearToSrgb()`: Color space conversion (if needed for alpha compositing)

## Data Model Design

### Core Type Definitions
**Location**: `wow-converter/src/lib/formats/icon/icon-types.ts`

```typescript
// Size options
export type IconSize = '64x64' | '128x128' | '256x256' | 'original';

// Style options  
export type IconStyle = 'classic-sd' | 'reforged-hd' | 'classic-hd-2.0';

// Frame/border types
export type IconFrame = 'btn' | 'disbtn' | 'pas' | 'dispas' | 'atc' | 'disatc' | 'none';

// Extras configuration
export interface IconExtras {
  readonly crop?: boolean;           // Apply 10% symmetric crop
  readonly blackFrame?: boolean;     // Apply black frame overlay
  readonly heroFrame?: boolean;       // Apply hero frame overlay
  readonly alpha?: boolean;           // Remove colors from transparent pixels (default: true)
}

// Main conversion options
export interface IconConversionOptions {
  readonly size?: IconSize;          // Output size, default: '256x256'
  readonly style?: IconStyle;         // Frame style, default: 'classic-sd'
  readonly frame?: IconFrame;         // Frame type, default: 'btn'
  readonly extras?: IconExtras;       // Extra processing options
}

// Internal processing state
export interface IconProcessingState {
  readonly inputImage: sharp.Sharp;   // Input image loaded in sharp
  readonly originalSize: { width: number; height: number };
  readonly targetSize: { width: number; height: number };
  readonly effectiveSize: IconSize;   // Size used for frame assets (closest match if 'original')
  readonly options: Required<IconConversionOptions>;
}

// Frame cache entry
export interface CachedFrame {
  readonly key: string;                // Cache key: `${size}-${style}-${frame}`
  readonly image: Buffer;              // Loaded frame PNG buffer
  readonly size: { width: number; height: number };
}

// Size mapping constants
export const SIZE_MAPPING: Readonly<Record<Exclude<IconSize, 'original'>, { width: number; height: number }>> = {
  '64x64': { width: 64, height: 64 },
  '128x128': { width: 128, height: 128 },
  '256x256': { width: 256, height: 256 },
} as const;

// Style folder name mapping
export const STYLE_FOLDER_MAP: Readonly<Record<IconStyle, string>> = {
  'classic-sd': 'ClassicSD',
  'reforged-hd': 'ReforgedHD',
  'classic-hd-2.0': 'ClassicHD2.0',
} as const;

// Frame file name mapping
export const FRAME_FILE_MAP: Readonly<Record<IconFrame, string>> = {
  'btn': 'BTN',
  'disbtn': 'DISBTN',
  'pas': 'PAS',
  'dispas': 'DISPAS',
  'atc': 'ATC',
  'disatc': 'DISATC',
  'none': 'NONE',
} as const;

// Frames that require desaturation in HD style
export const HD_DESATURATION_FRAMES: ReadonlySet<IconFrame> = new Set([
  'disbtn',
  'dispas',
  'disatc',
]);

// Default options
export const DEFAULT_ICON_OPTIONS: Required<IconConversionOptions> = {
  size: '256x256',
  style: 'classic-sd',
  frame: 'btn',
  extras: {
    crop: false,
    blackFrame: false,
    heroFrame: false,
    alpha: true,
  },
} as const;
```

## File Structure
```
wow-converter/
├── resources/
│   └── icon-frames/            # Frame PNG assets
│       ├── 64x64/
│       │   ├── ClassicSD/
│       │   ├── ReforgedHD/
│       │   └── ClassicHD2.0/
│       ├── 128x128/
│       └── 256x256/
└── src/lib/formats/icon/
    ├── convert-png-to-icon.ts    # Phase 1: PNG generation
    ├── icon-processor.ts          # Image processing pipeline
    ├── icon-types.ts              # Type definitions
    └── icon-utils.ts              # Utility functions (includes frame path resolution)

wow-converter/src/server/controllers/
└── convert-icon-to-blp.ts     # Phase 2: BLP conversion API
```

## Implementation Details

### Dependencies
- **sharp**: Already in package.json, use for image processing
- **path**: Node.js built-in for path resolution
- **fs**: Node.js built-in for file operations

### Key Challenges
1. **Alpha Compositing**: Sharp supports alpha compositing, but may need to verify linear vs sRGB blending matches Reforgerator behavior
2. **Frame Asset Loading**: Efficiently load and cache frame PNG buffers from assets directory
3. **PNG Buffer Processing**: All operations work on PNG buffers using sharp - no BLP until phase 2
4. **Output Path Management**: Ensure PNG icons are saved to correct wow.export asset directory structure
5. **API Integration**: Design clean API for phase 2 BLP conversion that reads from wow.export and writes to exported-assets

## Testing Strategy

### Phase 1 Testing (PNG Generation)
1. Test `convertPngToIcon()` with single PNG input → verify PNG buffer output
2. Test `generateIconPng()` → verify PNG saved to wow.export asset dir
3. Test all frame types (BTN, DISBTN, etc.)
4. Test all styles (ClassicSD, ReforgedHD, ClassicHD2.0)
5. Test all sizes (64x64, 128x128, 256x256, original)
6. Test extras (crop, black frame, hero frame, alpha)
7. Verify PNG files are viewable in browse-texture UI

### Phase 2 Testing (BLP Conversion)
1. Test API endpoint with valid PNG path
2. Verify BLP conversion using existing `pngsToBlps()` infrastructure
3. Verify BLP files saved to exported-assets directory
4. Compare output BLP files with Reforgerator output for visual parity

## Integration Points

### Phase 1: PNG Icon Generation
- **Function**: `generateIconPng(inputPng, outputPath, options)`
- **Output**: Saves processed PNG to `wow.export` asset directory
- **Usage**: Called during character/item export workflow
- **UI**: User can review PNG icons in browse-texture UI

### Phase 2: BLP Conversion
- **API Endpoint**: `POST /api/convert-icon-to-blp`
- **Input**: PNG path (relative to wow.export asset dir)
- **Output**: BLP file saved to `exported-assets` directory
- **Usage**: Client calls API after user approves PNG icons
- **Implementation**: Uses existing `pngsToBlps()` infrastructure from `formats/blp/blp.ts`

## Implementation Todos

1. Copy frame PNG assets from Reforgerator/data/frames/ to wow-converter/resources/icon-frames/
2. Create icon-types.ts with type definitions and constants
3. Create icon-utils.ts with helper functions
4. Create icon-processor.ts implementing PNG processing pipeline
5. Create convert-png-to-icon.ts with convertPngToIcon() and generateIconPng() functions
6. Create convert-icon-to-blp.ts API controller for phase 2 BLP conversion
7. Test PNG icon generation and saving to wow.export asset dir
8. Test API endpoint for PNG to BLP conversion

