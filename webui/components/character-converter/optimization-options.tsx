import { TriangleAlert } from 'lucide-react';

import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import {
  Optimization, TextureBakingForm,
} from '@/lib/models/export-character.model';

import { TooltipHelp } from '../common/tooltip-help';
import { Alert, AlertDescription } from '../ui/alert';
import { Button } from '../ui/button';
import {
  Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger,
} from '../ui/dialog';
import { Input } from '../ui/input';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '../ui/select';
import { Switch } from '../ui/switch';

const tooltips = {
  allMaterialsUnshaded: 'Force all materials to be marked as Unshaded.',
  maxTextureSize: 'Downscale texture images that are larger than this value; smaller ones stay unchanged. Select "Original" to leave textures size unchanged. This is useful to reduce file size, but will decrease visual quality when looking closely. 256px is acceptable for most unit models in Warcraft 3\'s RTS top-down view.',
  bakeTextures: 'Turn this on for advanced WoW models whose flames, glows, or scrolling patterns look wrong in Warcraft III — for example Firehawk flames. Off is faster and keeps files small; most models do not need it.',
  bakeEnabled: 'Recombine the model\'s special effects into Warcraft III textures. Export takes longer and the download gets larger, but advanced models look much closer to WoW.',
  bakeAnimate: 'Capture moving glows and scrolls as a short looping clip. Turn this off to bake one still picture instead — smaller files, but flames and energy will not flow.',
  bakeFps: 'How many pictures per second of moving glow to store. Higher is smoother and much larger. 15 is the default; 21 is closer to WoW and a bigger file.',
  bakeWindow: 'How many seconds of the effect loop to capture, then repeat. Longer captures more of a slow pattern and grows the file quickly. 4 seconds is usually enough; 0.5 seconds keeps the file small.',
  bakeResolution: 'Half resolution renders each baked texture at 50% width and height, using about one-quarter as many pixels. It keeps the selected FPS and capture duration unchanged.',
};

function bakeTexturesLabel(baking: TextureBakingForm): string {
  if (!baking.enabled) {
    return 'Off';
  }
  if (!baking.animate) {
    return 'Still';
  }
  return 'On';
}

export function BakeTexturesButton({ textureBaking, setTextureBaking }: {
  textureBaking: TextureBakingForm
  setTextureBaking: (value: TextureBakingForm) => void
}) {
  const loopSeconds = textureBaking.windowMS / 1000;

  return (
    <div className="flex items-center gap-2">
      <Dialog>
        <DialogTrigger asChild>
          <Button variant="outline" size="sm">
            Advanced Textures: {bakeTexturesLabel(textureBaking)}
          </Button>
        </DialogTrigger>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Advanced Textures</DialogTitle>
            <DialogDescription>
              Optional last-mile pass for advanced WoW effects that Warcraft III cannot display on its own, such as Firehawk flames.
            </DialogDescription>
          </DialogHeader>
          <Alert className="border-yellow-500/50 bg-yellow-500/10 py-3 text-yellow-800 dark:text-yellow-300 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400">
            <TriangleAlert className="h-4 w-4" />
            <AlertDescription>
              Turning this on significantly increases file size. Check the size after export to see if it is suitable for you. Increase FPS/Loop length to get closer to WoW quality. Decrease them to get smaller files.
            </AlertDescription>
          </Alert>
          <div className="space-y-4">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="bakeEnabled" className="text-sm flex items-center gap-2">
                Enable baking
                <TooltipHelp tooltips={tooltips.bakeEnabled}/>
              </Label>
              <Switch
                id="bakeEnabled"
                checked={textureBaking.enabled}
                onCheckedChange={(checked) => setTextureBaking({ ...textureBaking, enabled: checked })}
              />
            </div>
            <div
              className={`space-y-4 ${textureBaking.enabled ? '' : 'invisible'}`}
              aria-hidden={!textureBaking.enabled}
            >
              <div className="space-y-2">
                <Label htmlFor="bakeResolution" className="text-sm flex items-center gap-2">
                  Texture resolution
                  <TooltipHelp tooltips={tooltips.bakeResolution}/>
                </Label>
                <Select
                  value={String(textureBaking.resolutionScale)}
                  disabled={!textureBaking.enabled}
                  onValueChange={(value) => setTextureBaking({
                    ...textureBaking,
                    resolutionScale: value === '0.5' ? 0.5 : 1,
                  })}
                >
                  <SelectTrigger id="bakeResolution">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0.5">Half resolution (50% width and height)</SelectItem>
                    <SelectItem value="1">Full resolution</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Half resolution uses about one-quarter as many pixels. FPS and capture duration stay unchanged.
                </p>
              </div>
              <div className="flex items-center space-x-2">
                <Checkbox
                  id="bakeAnimate"
                  disabled={!textureBaking.enabled}
                  checked={textureBaking.animate}
                  onCheckedChange={(checked) => setTextureBaking({ ...textureBaking, animate: checked === true })}
                />
                <Label htmlFor="bakeAnimate" className="text-sm flex items-center gap-2">
                  Animate effects
                  <TooltipHelp tooltips={tooltips.bakeAnimate}/>
                </Label>
              </div>
              <div
                className={`grid grid-cols-2 gap-3 ${textureBaking.animate ? '' : 'invisible'}`}
                aria-hidden={!textureBaking.enabled || !textureBaking.animate}
              >
                <div className="space-y-2">
                  <Label htmlFor="bakeFps" className="text-sm flex items-center gap-2">
                    Frames per second
                    <TooltipHelp tooltips={tooltips.bakeFps}/>
                  </Label>
                  <Input
                    id="bakeFps"
                    type="number"
                    min={1}
                    max={60}
                    disabled={!textureBaking.enabled || !textureBaking.animate}
                    value={textureBaking.fps}
                    onChange={(e) => {
                      const fps = Math.min(60, Math.max(1, Number(e.target.value) || 1));
                      setTextureBaking({ ...textureBaking, fps });
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="bakeWindow" className="text-sm flex items-center gap-2">
                    Loop length (seconds)
                    <TooltipHelp tooltips={tooltips.bakeWindow}/>
                  </Label>
                  <Input
                    id="bakeWindow"
                    type="number"
                    min={0.5}
                    max={60}
                    step={0.5}
                    disabled={!textureBaking.enabled || !textureBaking.animate}
                    value={loopSeconds}
                    onChange={(e) => {
                      const seconds = Math.min(60, Math.max(0.5, Number(e.target.value) || 0.5));
                      setTextureBaking({ ...textureBaking, windowMS: Math.round(seconds * 1000) });
                    }}
                  />
                </div>
              </div>
            </div>
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button">Done</Button>
            </DialogClose>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <TooltipHelp tooltips={tooltips.bakeTextures}/>
    </div>
  );
}

export function OptimizationOptions({
  optimization, setOptimization, textureBaking, setTextureBaking,
}: {
  optimization: Optimization
  setOptimization: (optimization: Optimization) => void
  textureBaking: TextureBakingForm
  setTextureBaking: (value: TextureBakingForm) => void
}) {
  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Optimization Options</h3>
      <div className="flex flex-wrap items-center gap-4">
        <BakeTexturesButton textureBaking={textureBaking} setTextureBaking={setTextureBaking} />
        <div className="flex items-center space-x-2">
          <Checkbox
            id="allMaterialsUnshaded"
            checked={optimization.allMaterialsUnshaded ?? false}
            onCheckedChange={(checked) => setOptimization({ ...optimization, allMaterialsUnshaded: checked as boolean })
            }
          />
          <Label htmlFor="allMaterialsUnshaded" className="text-sm flex items-center gap-2">
            All Materials Unshaded
            <TooltipHelp tooltips={tooltips.allMaterialsUnshaded}/>
          </Label>
        </div>
        <div className="flex items-center space-x-2 gap-2">
          <Label htmlFor="maxTextureSize" className="text-sm flex items-center gap-2">
            Max Texture Size
            <TooltipHelp tooltips={tooltips.maxTextureSize}/>
          </Label>
          <Select
            value={optimization.maxTextureSize || 'none'}
            onValueChange={(value) => setOptimization({ ...optimization, maxTextureSize: value === 'none' ? undefined : value as '256' | '512' | '1024' })}
          >
            <SelectTrigger className="w-24">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Original</SelectItem>
              <SelectItem value="1024">1024px</SelectItem>
              <SelectItem value="512">512px</SelectItem>
              <SelectItem value="256">256px</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  );
}
