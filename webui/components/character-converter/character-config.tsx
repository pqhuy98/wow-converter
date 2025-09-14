import { Plus, Trash2, User } from 'lucide-react';
import Link from 'next/link';

import { Button } from '@/components/ui/button';
import {
  Card, CardContent,
  CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import { Character } from '@/lib/models/export-character.model';

import { BasicCharacterConfig } from '../common/basic-character-config';
import { TooltipHelp } from '../common/tooltip-help';
import { RefInput } from './ref-input';

const tooltips = {
  baseModel: 'The base character model to use. Can be a Wowhead URL, local file inside wow.export folder, or Display ID number.',
  mount: 'The mount model to use, can be a Wowhead URL, local file inside wow.export folder, or Display ID number. The mount model must have attachment point "Shield" - WoW uses it to attach the rider; and the character must have mount animation.',
  mountRemove: 'Remove the mount',
  mountScale: 'Additional scale multiplier of the mount model. Firstly the mount model is scaled equivalently to the character model, then this multiplier is applied. E.g. 1.0 = no change, 0.5 = half size, 2.0 = double size.',
  mountType: <span>
    The character's mount animation to use. For most cases, you should use "Mount".
    But some mount models need different animation, e.g. {' '}
    <Link href="https://www.wowhead.com/item=44554/flying-carpet" target="_blank" className="text-blue-500 hover:underline">
      Flying Carpet
    </Link>
      {' needs "MountCrouch", '}
    <Link href="https://www.wowhead.com/wotlk/item=50818/sky-golem" target="_blank" className="text-blue-500 hover:underline">
      Sky Golem
    </Link>
      {' needs "ReclinedMount", '}
    <Link href="https://www.wowhead.com/wotlk/item=50818/amani-bear" target="_blank" className="text-blue-500 hover:underline">
      Amani Bear
    </Link>
      {' needs "MountWide", '}

    <Link href="https://www.wowhead.com/spell=428013/incognitro-the-indecipherable-felcycle" target="_blank" className="text-blue-500 hover:underline">
      Incognito
    </Link>
      {' needs "MountChopper".'}
  </span>,
  seatOffsetForward: 'The forward (horizontal) offset of the seat. Use this field if you want to adjust the rider\'s seat position. Positive values move the seat forward, negative values move the seat backward.',
  seatOffsetUpward: 'The upward (vertical) offset of the seat. Use this field if you want to adjust the rider\'s seat position. Positive values move the seat upward, negative values move the seat downward.',
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

      <BasicCharacterConfig character={character} setCharacter={setCharacter} />

      <hr className="my-4" />

      {character.mount ? <div className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-6 gap-4">
          <div className="space-y-2 col-span-5">
            <RefInput
              value={character.mount?.path || { type: 'wowhead', value: '' }}
              onChange={(mountRef) => setCharacter((prev) => ({
                ...prev,
                mount: {
                  ...prev.mount, path: mountRef, seatOffset: [0, 0, 0], scale: undefined,
                },
              }))}
              label={
                <div className="flex items-center gap-2 w-full">
                  <div className="flex items-center gap-2">
                    <span>Mount Model</span>
                    <TooltipHelp tooltips={tooltips.mount}/>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setCharacter((prev) => ({ ...prev, mount: undefined }))}
                    className="text-red-500 hover:text-red-700 h-6 w-6 p-0"
                  >
                    <TooltipHelp trigger={<Trash2 className="h-3 w-3" />} tooltips={tooltips.mountRemove}/>
                  </Button>
                </div>
              }
              category="mount"
            />
          </div>

          <div className="space-y-2 col-span-1">
            <div className="flex items-center gap-2">
              <Label className="text-sm">Scale</Label>
              <TooltipHelp tooltips={tooltips.mountScale}/>
            </div>
            <Input
              id="mountScale"
              type="number"
              step="0.1"
              placeholder="1.0"
              value={character.mount?.scale || ''}
              disabled={character.mount?.path.value === ''}
              className="flex-1"
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
          <div className="flex items-center space-x-2 col-span-2">
            <Label htmlFor="mountType" className="flex items-center gap-2 text-sm">
              Type
              <TooltipHelp tooltips={tooltips.mountType}/>
            </Label>
            <Select
              value={character.mount?.animation || 'Mount'}
              onValueChange={(value) => setCharacter((prev) => ({
                ...prev,
                mount: { ...prev.mount!, animation: value },
              }))}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select type" />
              </SelectTrigger>
              <SelectContent align="start">
                <SelectItem value="Mount">Mount</SelectItem>
                <SelectItem value="MountCrouch">Crouch</SelectItem>
                <SelectItem value="MountWide">Wide</SelectItem>
                <SelectItem value="MountChopper">Chopper</SelectItem>
                <SelectItem value="ReclinedMount">Reclined</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center space-x-2 col-span-2">
            <Label htmlFor="seatOffsetForward" className="flex items-center gap-2 text-sm">
              Forward
              <TooltipHelp tooltips={tooltips.seatOffsetForward}/>
            </Label>
            <Input
              id="seatOffsetForward"
              type="number"
              step="0.1"
              placeholder="0"
              className="flex-1"
              value={character.mount?.seatOffset?.at(0) || ''}
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

          <div className="flex items-center space-x-2 col-span-2">
            <Label htmlFor="seatOffsetUpward" className="flex items-center gap-2 text-sm">
              Upward
              <TooltipHelp tooltips={tooltips.seatOffsetUpward}/>
            </Label>
            <Input
              id="seatOffsetUpward"
              type="number"
              step="0.1"
              placeholder="0"
              className="flex-1"
              value={character.mount?.seatOffset?.at(2) || ''}
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
        className="w-full bg-transparent"
        onClick={() => setCharacter((prev) => ({
          ...prev,
          mount: {
            path: { type: 'wowhead', value: 'https://www.wowhead.com/wotlk/item=50818/invincibles-reins' },
            seatOffset: [-15, 0, 15],
          },
        }))}
        >
          <Plus className="h-4 w-4 mr-2" />
          Add Mount
        </Button>
      }
    </CardContent>
  </Card>;
}
