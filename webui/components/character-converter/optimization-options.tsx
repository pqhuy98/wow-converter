import { HelpCircle } from 'lucide-react';

import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import { Optimization } from '@/lib/models/export-character.model';

import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '../ui/select';

const tooltips = {
  sortSequences: 'Sort animations by name in the order of: Stand, Walk, Attack, Spell, Death, Decay, Cinematic XXX.',
  removeUnusedVertices: 'Remove geoset vertices that are not used by any geoset faces.',
  removeUnusedNodes: 'Remove nodes that are not used in any geosets or do not contain used children nodes.',
  removeUnusedMaterials: 'Remove materials and textures that are not used in any geosets.',
  optimizeKeyFrames: 'Remove key frames that are not used in any animation, or are insignificant.',
  maxTextureSize: 'Downscale texture images that are larger than this value; smaller ones stay unchanged. Select "Original" to leave textures size unchanged. This is useful to reduce file size, but will decrease visual quality when looking closely. 256px is acceptable for most unit models in Warcraft 3\'s RTS top-down view.',
};

export function OptimizationOptions({ optimization, setOptimization }: {
  optimization: Optimization
  setOptimization: (optimization: Optimization) => void
}) {
  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Optimization Options</h3>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
        <div className="flex items-center space-x-2">
          <Checkbox
            id="sortSequences"
            checked={optimization.sortSequences}
            onCheckedChange={(checked) => setOptimization({ ...optimization, sortSequences: checked as boolean })
            }
          />
          <Label htmlFor="sortSequences" className="text-sm flex items-center gap-2">
            Sort Sequences
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.sortSequences}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </Label>
        </div>

        <div className="flex items-center space-x-2">
          <Checkbox
            id="removeUnusedVertices"
            checked={optimization.removeUnusedVertices}
            onCheckedChange={(checked) => setOptimization({ ...optimization, removeUnusedVertices: checked as boolean })
            }
          />
          <Label htmlFor="removeUnusedVertices" className="text-sm flex items-center gap-2">
            Remove Unused Vertices
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.removeUnusedVertices}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </Label>
        </div>

        <div className="flex items-center space-x-2">
          <Checkbox
            id="removeUnusedNodes"
            checked={optimization.removeUnusedNodes}
            onCheckedChange={(checked) => setOptimization({ ...optimization, removeUnusedNodes: checked as boolean })
            }
          />
          <Label htmlFor="removeUnusedNodes" className="text-sm flex items-center gap-2">
            Remove Unused Nodes
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.removeUnusedNodes}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </Label>
        </div>

        <div className="flex items-center space-x-2">
          <Checkbox
            id="removeUnusedMaterials"
            checked={optimization.removeUnusedMaterialsTextures}
            onCheckedChange={(checked) => setOptimization({ ...optimization, removeUnusedMaterialsTextures: checked as boolean })
            }
          />
          <Label htmlFor="removeUnusedMaterials" className="text-sm flex items-center gap-2">
            Optimize Materials
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.removeUnusedMaterials}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </Label>
        </div>

        <div className="flex items-center space-x-2">
          <Checkbox
            id="optimizeKeyFrames"
            disabled
            checked={true}
          />
          <Label htmlFor="optimizeKeyFrames" className="text-sm flex items-center gap-2">
            Optimize Key Frames
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.optimizeKeyFrames}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </Label>
        </div>
      </div>

      <div className="flex items-center space-x-2 gap-2">
        <Label htmlFor="maxTextureSize" className="text-sm flex items-center gap-2">
          Max Texture Size
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger>
                <HelpCircle className="h-4 w-4 text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>
                <p className="max-w-xs">{tooltips.maxTextureSize}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
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
  );
}
