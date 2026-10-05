'use client';

import { Settings } from 'lucide-react';

import { BakeTexturesButton } from '@/components/character-converter/optimization-options';
import { BasicCharacterConfig } from '@/components/common/basic-character-config';
import { TooltipHelp } from '@/components/common/tooltip-help';
import { Button } from '@/components/ui/button';
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import {
  Character, ModelFormat, ModelFormatVersion, Optimization, TextureBakingForm,
} from '@/lib/models/export-character.model';

const tooltips = {
  format: 'Model format (MDX vs MDL). MDX is the binary format, the file is most compact and lowest file size. MDL is the text format for debugging purposes, the file is human readable when opened in text editors, but has larger file size.',
  formatVersion: "Model format version (HD vs SD). HD models work in all Warcraft 3 Retail's Reforged and Classic graphics modes, it has the highest fidelity with precise WoW model data. However HD models cannot be opened in pre-Reforged legacy 3D modeling softwares. If you want to use those legacy tools for post-processing, choose SD 800 instead.",
};

export function SettingsDialogButton({
  character,
  setCharacter,
  outputFileName: _outputFileName,
  setOutputFileName: _setOutputFileName,
  format,
  setFormat,
  formatVersion,
  setFormatVersion,
  optimization: _optimization,
  setOptimization: _setOptimization,
  textureBaking,
  setTextureBaking,
  disabled,
  className,
}: {
  character: Character
  setCharacter: React.Dispatch<React.SetStateAction<Character>>
  outputFileName: string
  setOutputFileName: React.Dispatch<React.SetStateAction<string>>
  format: ModelFormat
  setFormat: React.Dispatch<React.SetStateAction<ModelFormat>>
  formatVersion: ModelFormatVersion
  setFormatVersion: React.Dispatch<React.SetStateAction<ModelFormatVersion>>
  optimization: Optimization
  setOptimization: React.Dispatch<React.SetStateAction<Optimization>>
  textureBaking: TextureBakingForm
  setTextureBaking: React.Dispatch<React.SetStateAction<TextureBakingForm>>
  disabled?: boolean
  className?: string
  }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="icon" title="Export settings" disabled={disabled} className={className}>
          <Settings className="h-4 w-4" />
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Browse Model Settings</DialogTitle>
          <DialogDescription>Configure model export settings when browsing</DialogDescription>
        </DialogHeader>

        <BasicCharacterConfig character={character} setCharacter={setCharacter} />

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label className="text-sm flex items-center gap-2">
              Export Format
              <TooltipHelp tooltips={tooltips.format}/>
            </Label>
            <Select value={format} onValueChange={(value: ModelFormat) => setFormat(value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                <SelectItem value="mdx">.mdx</SelectItem>
                <SelectItem value="mdl">.mdl</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label className="text-sm flex items-center gap-2">
              Model Version
              <TooltipHelp tooltips={tooltips.formatVersion}/>
            </Label>
            <Select value={formatVersion} onValueChange={(value: ModelFormatVersion) => setFormatVersion(value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                <SelectItem value="1000">1000 (HD)</SelectItem>
                <SelectItem value="800">800 (SD)</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <BakeTexturesButton textureBaking={textureBaking} setTextureBaking={setTextureBaking} />
      </DialogContent>
    </Dialog>
  );
}
