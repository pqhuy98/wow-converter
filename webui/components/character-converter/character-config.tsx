import { HelpCircle, User } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Card, CardContent,
  CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import { AttackTag, Character, ModelSize } from '@/lib/models/export-character.model';

import { RefInput } from './ref-input';

const attackTagOptions: { value: AttackTag | 'all', label: string, description: string }[] = [
  { value: 'Auto', label: 'Auto', description: 'Detect attack tag from weapons' },
  { value: 'all', label: 'All', description: 'Include all attack animations' },
  { value: '1H', label: '1H Weapon', description: 'The model uses 1H weapon(s)' },
  { value: '2H', label: '2H Weapon', description: 'The model uses a 2H weapon' },
  { value: '2HL', label: '2HL Weapon', description: 'The model uses a 2H polearm' },
  { value: 'Unarmed', label: 'Unarmed', description: 'The model uses fists and kicks' },
  { value: 'Bow', label: 'Bow', description: 'The model uses a bow.' },
  { value: 'Rifle', label: 'Rifle', description: 'The model uses a rifle.' },
  { value: 'Thrown', label: 'Thrown', description: 'The model uses a thrown weapon.' },
];

const sizeOptions: { value: ModelSize | 'none', label: string, description: string }[] = [
  { value: 'none', label: 'Default', description: 'Original WoW size times 56' },
  { value: 'small', label: 'Small', description: 'As tall as Undead Ghoul' },
  { value: 'medium', label: 'Medium', description: 'As tall as Orc Grunt' },
  { value: 'large', label: 'Large', description: 'As tall as Undead Abomination' },
  { value: 'hero', label: 'Hero', description: 'As tall as Tauren Chieftain' },
  { value: 'giant', label: 'Giant', description: 'As tall as Flesh Golem' },
];

const tooltips = {
  baseModel: 'The base character model to use. Can be a Wowhead URL, local file inside wow.export folder, or Display ID number.',
  attackAnimation: 'Determines which attack animations the character will use.',
  characterSize: 'How tall the character is in the game.',
  movementSpeed: 'Animation - walk speed ("uwal") of the unit in World Editor. The tool will try to slow down/speed up the Walk animations to match the Warcraft movement speed. If you experience a bug with too fast or too slow walk animation, set to 0 to keep the original WoW animation speed.',
  scaleMultiplier: 'Additional scale multiplier, optional. E.g. 1.0 = no change, 0.5 = half size, 2.0 = double size.',
  keepCinematic: 'Preserve cinematic animation sequences in the exported model. Warning: WoW models have many cinematic sequences, this significantly increases file size.',
  noDecay: 'Do not automatically add Decay animations.',
  particleDensity: 'Particle density, e.g. 1.0 = default, 0.5 = half, 2.0 = double, 0 = none... Higher density will decrease in-game FPS due to more particles.',
  portraitCamera: 'Name of the sequence to use for positioning the character portrait camera. E.g. if later you use Stand Ready as default stand animation, the portrait camera needs to be placed lower since the model will usually hunch a bit.',
  mount: 'The mount model to use, can be a Wowhead URL, local file inside wow.export folder, or Display ID number. The mount model must have attachment point "Shield" - WoW uses it to attach the rider. If mount is provided, the character must have animation "Mount".',
  mountScale: 'The scale of the mount model. E.g. 1.0 = no change, 0.5 = half size, 2.0 = double size.',
  seatOffsetForward: 'The forward (horizontal) offset of the seat. Use this field if you want to adjust the rider\'s seat position. E.g. 0 = no change, 10 = 10 units forward, -10 = 10 units backward.',
  seatOffsetUpward: 'The upward (vertical) offset of the seat. Use this field if you want to adjust the rider\'s seat position. E.g. 0 = no change, 10 = 10 units upward, -10 = 10 units downward.',
};

export function CharacterConfig({
  character,
  setCharacter,
  clearOutputFileName,
}: {
  character: Character
  setCharacter: React.Dispatch<React.SetStateAction<Character>>
  clearOutputFileName?: () => void
}) {
  const [particlesDensity, setParticlesDensity] = useState(character.particlesDensity);

  return <Card>
    <CardHeader className="pb-4">
      <CardTitle className="flex items-center gap-2 text-lg">
        <User className="h-5 w-5" />
        Character Configuration
      </CardTitle>
      <CardDescription>Configure the base character model and its properties</CardDescription>
    </CardHeader>
    <CardContent className="space-y-4">
      <RefInput
        value={character.base}
        onChange={(base) => {
          setCharacter((prev) => {
            let attachItems = prev.attachItems;
            if (prev.base.type === 'wowhead' && base.type === 'wowhead') {
              if (!prev.base.value.includes('dressing-room') && base.value.includes('dressing-room')) {
                clearOutputFileName?.();
                attachItems = {};
              }
            }
            return { ...prev, base, attachItems };
          });
        }}
        label="Base Model"
        tooltip={tooltips.baseModel}
        category="npc"
      />

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <Label className="text-sm">Attack Animation</Label>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.attackAnimation}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <Select
            value={character.attackTag || 'all'}
            onValueChange={(value: AttackTag | 'all') => setCharacter((prev) => ({
              ...prev,
              attackTag: value === 'all' ? undefined : (value as AttackTag),
            }))
            }
          >
            <SelectTrigger>
              <SelectValue placeholder="Select attack type" />
            </SelectTrigger>
            <SelectContent align="start">
              {attackTagOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  <div className="flex flex-col">
                    <span className="text-left">{option.label}</span>
                    <span className="text-xs text-muted-foreground">{option.description}</span>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <Label className="text-sm">Character Size</Label>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <HelpCircle className="h-4 w-4 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="max-w-xs">{tooltips.characterSize}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <Select
            value={character.size || 'none'}
            onValueChange={(value: string) => setCharacter((prev) => ({
              ...prev,
              size: value === 'none' ? undefined : (value as ModelSize),
            }))
            }
          >
            <SelectTrigger>
              <SelectValue placeholder="Select size" />
            </SelectTrigger>
            <SelectContent align="start">
              {sizeOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  <div className="flex flex-col">
                    <span className="text-left">{option.label}</span>
                    <span className="text-xs text-muted-foreground">{option.description}</span>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="flex items-center gap-3">
          <Label htmlFor="movespeed" className="text-sm min-w-fit">
            Animation Walk Speed
          </Label>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger>
                <HelpCircle className="h-4 w-4 text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>
                <p className="max-w-xs">{tooltips.movementSpeed}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <Input
            id="movespeed"
            type="number"
            step="1"
            value={character.inGameMovespeed || ''}
            onChange={(e) => setCharacter((prev) => ({ ...prev, inGameMovespeed: Number.parseInt(e.target.value, 10) || 0 }))
            }
            className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
          />
        </div>

        <div className="flex items-center gap-3">
          <Label htmlFor="scale" className="text-sm min-w-fit">
            Scale Multiplier
          </Label>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger>
                <HelpCircle className="h-4 w-4 text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>
                <p className="max-w-xs">{tooltips.scaleMultiplier}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <Input
            id="scale"
            type="number"
            step="0.1"
            placeholder="1.0"
            value={character.scale || ''}
            onChange={(e) => setCharacter((prev) => ({ ...prev, scale: Number.parseFloat(e.target.value) || undefined }))
            }
            className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
          />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="space-y-3">
          <div className="flex items-center space-x-2">
            <Checkbox
              id="keepCinematic"
              checked={character.keepCinematic || false}
              onCheckedChange={(checked) => setCharacter((prev) => ({ ...prev, keepCinematic: checked as boolean }))}
            />
            <Label htmlFor="keepCinematic" className="flex items-center gap-2 text-sm">
              Keep Cinematic Animations
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-xs">{tooltips.keepCinematic}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </Label>
          </div>

          {/* <div className="flex items-center space-x-2">
            <Checkbox
              id="noDecay"
              checked={character.noDecay || false}
              onCheckedChange={(checked) => setCharacter((prev) => ({ ...prev, noDecay: checked as boolean }))}
            />
            <Label htmlFor="noDecay" className="flex items-center gap-2 text-sm">
              No Decay
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-xs">{tooltips.noDecay}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </Label>
          </div> */}
        </div>

        <div className="space-y-2">
          <div className="flex items-center space-x-2">
            <Label htmlFor="noParticles" className="flex items-center gap-2 text-sm">
              Particle Density
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-xs">{tooltips.particleDensity}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </Label>
            <Input
              id="particlesDensity"
              type="number"
              placeholder="1.0"
              min={0}
              max={5}
              value={particlesDensity}
              onChange={(e) => {
                const value = Number.parseFloat(e.target.value);
                setParticlesDensity(value);
                setCharacter((prev) => ({ ...prev, particlesDensity: isNaN(value) ? 1 : value }));
              }}
              className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
            />
          </div>
        </div>
      </div>

      <hr className="my-4" />

      {character.mount ? <div className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-6 gap-4">
          <div className="space-y-2 col-span-5">
            <RefInput
              value={character.mount?.path || { type: 'wowhead', value: '' }}
              onChange={(mountRef) => setCharacter((prev) => ({
                ...prev,
                mount: { path: mountRef, scale: prev.mount?.scale },
              }))}
              label="Mount Model"
              tooltip={tooltips.mount}
              category="mount"
            />
          </div>

          <div className="space-y-2 col-span-1">
            <div className="flex items-center gap-2">
              <Label className="text-sm">Scale</Label>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-xs">{tooltips.mountScale}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            <Input
              id="mountScale"
              type="number"
              step="0.1"
              placeholder="1.0"
              value={character.mount?.scale || ''}
              disabled={character.mount?.path.value === ''}
              className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
              onChange={(e) => setCharacter((prev) => {
                if (!prev.mount) {
                  return prev;
                }
                if (prev.mount.path.value === '') {
                  return { ...prev, mount: undefined };
                }
                return {
                  ...prev,
                  mount: {
                    path: prev.mount.path,
                    scale: Number.parseFloat(e.target.value) || undefined,
                  },
                };
              })
              }
            />
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-6 gap-4">
          <div className="flex items-center space-x-2 col-span-3">
            <Label htmlFor="seatOffsetForward" className="flex items-center gap-2 text-sm">
              Seat Offset Forward
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-xs">{tooltips.seatOffsetForward}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </Label>
            <Input
              id="seatOffsetForward"
              type="number"
              step="0.1"
              placeholder="0"
              className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
              onChange={(e) => setCharacter((prev) => ({
                ...prev,
                mount: {
                  ...prev.mount!,
                  seatOffset: [
                    Number.parseFloat(e.target.value) || 0,
                    prev.mount?.seatOffset?.at(1) ?? 0,
                    prev.mount?.seatOffset?.at(2) ?? 0,
                  ],
                },
              }))}
            />
          </div>

          <div className="flex items-center space-x-2 col-span-3">
            <Label htmlFor="seatOffsetUpward" className="flex items-center gap-2 text-sm">
              Seat Offset Upward
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-xs">{tooltips.seatOffsetUpward}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </Label>
            <Input
              id="seatOffsetUpward"
              type="number"
              step="0.1"
              placeholder="0"
              className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
              onChange={(e) => setCharacter((prev) => ({
                ...prev,
                mount: {
                  ...prev.mount!,
                  seatOffset: [
                    prev.mount?.seatOffset?.at(0) ?? 0,
                    prev.mount?.seatOffset?.at(1) ?? 0,
                    Number.parseFloat(e.target.value) || 0,
                  ],
                },
              }))}
            />
          </div>
        </div>
      </div> : <Button variant="outline"
        onClick={() => setCharacter((prev) => ({ ...prev, mount: { path: { type: 'wowhead', value: '' } } }))}
        >
          Add Mount
        </Button>
      }
    </CardContent>
  </Card>;
}
