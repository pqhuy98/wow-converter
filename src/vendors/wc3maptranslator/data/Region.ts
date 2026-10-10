interface Region {
  cameraBlocker?: number
  alphaTileMinimapColor?: number // packed BGRA
  alpha?: number
  position: Rect
  name: string
  id: number
  weatherEffect: string
  ambientSound: string
  color: number[]
}

interface Rect {
  left: number
  bottom: number
  right: number
  top: number
}

export type { Region, Rect };
