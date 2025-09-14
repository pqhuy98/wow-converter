'use client';

import { Settings } from 'lucide-react';

import { BasicCharacterConfig } from '@/components/common/basic-character-config';
import { Button } from '@/components/ui/button';
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger,
} from '@/components/ui/dialog';
// (format/version/optimization UI removed for this dialog)
import {
  Character, ModelFormat, ModelFormatVersion, Optimization,
} from '@/lib/models/export-character.model';

export function SettingsDialogButton({
  character,
  setCharacter,
  // unused in minimal dialog but kept for API compatibility
  outputFileName: _outputFileName,
  setOutputFileName: _setOutputFileName,
  format: _format,
  setFormat: _setFormat,
  formatVersion: _formatVersion,
  setFormatVersion: _setFormatVersion,
  optimization: _optimization,
  setOptimization: _setOptimization,
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
      </DialogContent>
    </Dialog>
  );
}
