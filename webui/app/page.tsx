'use client';

import _ from 'lodash';
import { Download, HelpCircle, History } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';

import { host } from '@/app/config';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import { AttachItems } from '@/components/wow-converter/attach-items';
import { CharacterConfig } from '@/components/wow-converter/character-config';
import { isLocalRef, validateRef } from '@/components/wow-converter/ref-input';
import { setServerConfig } from '@/lib/config';
import {
  AttachItem, Character, commonAttachments, ExportRequest, JobStatus, ModelFormat, ModelFormatVersion, otherAttachments, RefSchema,
} from '@/lib/models/export-character.model';

import ModelViewerUi from './model-viewer';

// Tooltips organized in a record
const tooltips = {
  baseModel: 'The base character model to use. Can be a Wowhead URL, local file inside wow.export folder, or Display ID number.',
  attackAnimation: 'Determines which attack animations the character will use.',
  characterSize: 'How tall the character is in the game.',
  // eslint-disable-next-line max-len
  movementSpeed: 'Animation - walk speed ("uwal") of the unit in World Editor. The tool will try to slow down/speed up the Walk animations to match the Warcraft movement speed. If you experience a bug with too fast or too slow walk animation, set to 0 to keep the original WoW animation speed.',
  scaleMultiplier: 'Additional scale multiplier, optional. E.g. 1.0 = no change, 0.5 = half size, 2.0 = double size.',
  keepCinematic: 'Preserve cinematic animation sequences in the exported model. Warning: WoW models have many cinematic sequences, this significantly increases file size.',
  noDecay: 'Do not automatically add Decay animations.',
  particleDensity: 'Particle density (1.0 = default, 0.5 = half, 2.0 = double, 0 = none...). Putting higher density will decrease rendering performance.',
  // eslint-disable-next-line max-len
  portraitCamera: 'Name of the sequence to use for positioning the character portrait camera. E.g. if later you use Stand Ready as default stand animation, the portrait camera needs to be placed lower since the model will usually hunch a bit.',
  itemReference: 'The item to attach - can be a Wowhead URL, local file inside wow.export folder, or Display ID.',
  attachmentPoint: 'Where on the character model this item will be attached',
  itemScale: 'Additional scale multiplier for this specific item (1.0 = no change). Firstly the item will be scaled to match the character, then this multiplier will be applied.',
  sortSequences: 'Sort animations by name in the order of: Stand, Walk, Attack, Spell, Death, Decay, Cinematic XXX.',
  removeUnusedVertices: 'Remove geoset vertices that are not used by any geoset faces.',
  removeUnusedNodes: 'Remove nodes that are not used in any geosets or do not contain used children nodes.',
  removeUnusedMaterials: 'Remove materials and textures that are not used in any geosets.',
  optimizeKeyFrames: 'Remove key frames that are not used in any animation, or are insignificant.',
  // eslint-disable-next-line max-len
  format: 'Model format (MDX vs MDL). MDX is the binary format, the file is most compact and lowest file size. MDL is the text format for debugging purposes, the file is human readable when opened in text editors, at the cost of larger file size.',
  // eslint-disable-next-line max-len
  formatVersion: "Model format version (HD vs SD). HD models work in all Warcraft 3 Retail's Reforged and Classic graphics modes, it has the highest fidelity with precise WoW model data. However HD models cannot be opened in legacy modeling tools like Magos Model Editor. If you want to use those legacy tools for post-processing, choose SD 800 instead. WARNING: wow-converter might export very broken SD models on complex WoW models. SD conversion does not guarantee to work, after exporting you need to check if each animation is working.",
};

const defaultCharacter = {
  base: { type: 'wowhead', value: 'https://www.wowhead.com/wotlk/npc=36597/the-lich-king' },
  size: 'hero',
  attackTag: '2H',
  inGameMovespeed: 270,
  attachItems: {
    1: {
      path: { type: 'wowhead', value: 'https://www.wowhead.com/classic/item=231885/frostmourne' },
    },
  },
  portraitCameraSequenceName: 'Stand',
} as const;

export default function WoWNPCExporter() {
  useEffect(() => {
    void fetch(`${host}/export/character/config`)
      .then((res) => res.json())
      .then((config) => {
        setServerConfig(config);
      });
  }, []);

  const [character, setCharacter] = useState<Character>(_.cloneDeep(defaultCharacter));

  // Guess output file name from base model value
  const [outputFileName, setOutputFileName] = useState(guessOutputFileWowhead(character.base.value) ?? '');
  useEffect(() => {
    const guessed = guessOutputFile(character.base);
    if (guessed) {
      setOutputFileName(guessed);
    }
  }, [character.base.value]);

  const [format, setFormat] = useState<ModelFormat>('mdx');
  const [formatVersion, setFormatVersion] = useState<ModelFormatVersion>('1000');
  const [optimization, setOptimization] = useState({
    sortSequences: true,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true,
  });

  const [isExporting, setIsExporting] = useState(false);

  // Job/queue tracking
  const [jobStatus, setJobStatus] = useState<JobStatus | null>(null);
  const [viewerModelPath, setViewerModelPath] = useState<string | undefined>(undefined);

  useEffect(() => {
    const checkExportResult = async () => {
      const res = await fetch(`${host}/export/character/demos`);
      const jobs = await res.json();
      if (jobs.length > 0) {
        setViewerModelPath(jobs[Math.floor(Math.random() * jobs.length)].result.exportedModels[0]);
      }
    };
    void checkExportResult();
  }, []);

  const addAttachItem = () => {
    // Find the first unused attachment ID, starting with common ones
    const usedIds = new Set(Object.keys(character.attachItems || {}).map(Number));

    let newId = commonAttachments[0].id;
    for (const attachment of [...commonAttachments, ...otherAttachments]) {
      if (!usedIds.has(attachment.id)) {
        newId = attachment.id;
        break;
      }
    }

    setCharacter((prev) => ({
      ...prev,
      attachItems: {
        ...prev.attachItems,
        [newId]: {
          path: {
            type: 'wowhead',
            value: Object.keys(prev.attachItems || {}).length === 0
              ? defaultCharacter.attachItems[1].path.value
              : '',
          },
        },
      },
    }));
  };

  const removeAttachItem = (id: number) => setCharacter((prev) => {
    const newAttachItems = { ...prev.attachItems };
    delete newAttachItems[id];
    return { ...prev, attachItems: newAttachItems };
  });

  const updateAttachItem = (id: number, item: AttachItem) => setCharacter((prev) => ({
    ...prev,
    attachItems: {
      ...prev.attachItems,
      [id]: { ...item },
    },
  }));

  const checkExportValid = useCallback(() => {
    // Check base model
    if (validateRef(character.base, 'npc', true)) return 'Invalid base model';

    // Check output filename
    if (!outputFileName.trim()) return 'Output filename is required';
    if (!isLocalRef(outputFileName)) return 'Invalid output filename';

    // Check all attach items have valid references
    const attachItems = character.attachItems || {};
    for (const item of Object.values(attachItems)) {
      if (validateRef(item.path, 'item', true)) return 'Invalid attach item';
    }

    return null;
  }, [character, outputFileName]);

  const handleExport = async () => {
    const error = checkExportValid();
    if (error) {
      alert(error);
      return;
    }
    setIsExporting(true);
    setJobStatus(null);

    try {
      // Prepare request
      const exportCharacter = {
        ...character,
        attackTag: character.attackTag === undefined ? '' : character.attackTag,
      };

      const request: ExportRequest = {
        character: exportCharacter,
        outputFileName,
        optimization,
        format,
        formatVersion,
      };

      const response = await fetch(`${host}/export/character`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        throw new Error(await response.text());
      }

      const result = await response.json();
      setJobStatus(result);
    } catch (error: unknown) {
      console.error('Export error:', error);
      setJobStatus({
        id: '',
        status: 'failed',
        position: null,
        result: null,
        error: error instanceof Error ? error.message : String(error),
        submittedAt: Date.now(),
      });
    } finally {
      setIsExporting(false);
    }
  };

  // is window focused?
  const [isWindowFocused, setIsWindowFocused] = useState(true);
  useEffect(() => {
    const handleFocus = () => setIsWindowFocused(true);
    const handleBlur = () => setIsWindowFocused(false);
    window.addEventListener('focus', handleFocus);
    window.addEventListener('blur', handleBlur);
  }, []);

  const [doneCount, setDoneCount] = useState(0);

  // Poll job status every 1s when a job is active
  useEffect(() => {
    if (!jobStatus || jobStatus.status === 'done' || jobStatus.status === 'failed') return undefined;

    let pendingFetches = 0;
    const fetchJobStatus = async () => {
      try {
        pendingFetches++;
        if (pendingFetches > 1) return;
        const res = await fetch(`${host}/export/character/status/${jobStatus.id}`);
        if (!res.ok) {
          throw new Error(await res.text());
        }
        const data = await res.json();
        setJobStatus(data);

        if (data.status === 'pending') {
          // do nothing
        } else if (data.status === 'processing') {
          // do nothing
        } else if (data.status === 'done') {
          setDoneCount(doneCount + 1);
          setViewerModelPath(data.result.exportedModels[0]);
          clearInterval(interval);
        } else if (data.status === 'failed') {
          clearInterval(interval);
        }
      } catch (e: unknown) {
        console.error('Polling error:', e);
        setJobStatus({
          id: '',
          status: 'failed',
          position: null,
          result: null,
          error: e instanceof Error ? e.message : String(e),
          submittedAt: Date.now(),
        });
        clearInterval(interval);
      } finally {
        pendingFetches--;
      }
    };

    const interval = setInterval(() => void fetchJobStatus(), 1000);
    void fetchJobStatus();

    return () => {
      clearInterval(interval);
    };
  }, [jobStatus?.id, isWindowFocused]);

  /**
   * Download the exported assets as a ZIP by calling the new POST /download API.
   */
  const handleDownloadZip = async () => {
    if (!jobStatus?.result) return;

    const files = [
      ...(jobStatus.result.exportedModels || []),
      ...(jobStatus.result.exportedTextures || []),
    ];

    if (files.length === 0) {
      // eslint-disable-next-line no-alert
      alert('Nothing to download – exported files list is empty');
      return;
    }

    try {
      const res = await fetch(`${host}/download`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files }),
      });

      if (!res.ok) {
        throw new Error(await res.text());
      }

      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${outputFileName || 'export'}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (e: unknown) {
      console.error('Download ZIP error:', e);
      alert(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-4">
      <div className="max-w-6xl mx-auto space-y-4">
        {/* Navigation Header */}
        <div className="flex justify-between items-center mb-6">
          <div className="text-center flex-1">
            <h1 className="text-4xl font-bold text-gray-900">Huy's WOW-CONVERTER</h1>
            <p className="text-lg text-gray-600">Easily export WoW NPC models into Warcraft 3 MDL/MDX</p>
          </div>
          <Button
            variant="outline"
            onClick={() => window.location.href = '/recents'}
            className="flex items-center gap-2"
          >
            <History className="h-4 w-4" />
            Recent Exports
          </Button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <CharacterConfig
            character={character}
            setCharacter={setCharacter}
            tooltips={tooltips}
          />
          <AttachItems
            character={character}
            setCharacter={setCharacter}
            tooltips={tooltips}
            removeAttachItem={removeAttachItem}
            updateAttachItem={updateAttachItem}
            addAttachItem={addAttachItem}
          />
        </div>

        {/* Export Settings */}
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-lg">Export Configuration</CardTitle>
            <CardDescription>Configure output settings and optimizations</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-8 gap-4 items-end">
              <div className="space-y-2 md:col-span-3">
                <Label htmlFor="filename" className="text-sm">
                  Output File Name
                </Label>
                <Input
                  id="filename"
                  placeholder="my-character"
                  value={outputFileName}
                  onChange={(e) => setOutputFileName(e.target.value)}
                  className={`border-2 bg-white ${!outputFileName.trim() ? 'border-red-500' : 'border-gray-300 focus:border-blue-500'}`}
                />
              </div>

              <div className="space-y-2 md:col-span-1">
                <Label className="text-sm flex items-center gap-2">
                  Export Format
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.format}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
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

              <div className="space-y-2 md:col-span-1">
                <Label className="text-sm flex items-center gap-2">
                  Model Version
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.formatVersion}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </Label>
                <Select value={formatVersion} onValueChange={(value: ModelFormatVersion) => setFormatVersion(value)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="start">
                    <SelectItem value="1000">1000 (HD)</SelectItem>
                    <SelectItem value="800">800 (SD, experimental)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="md:col-span-3">
                <Button onClick={() => void handleExport()} disabled={isExporting || jobStatus?.status === 'pending' || jobStatus?.status === 'processing'} className="w-full" size="lg">
                  {isExporting ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                      Exporting...
                    </>
                  ) : (
                    <>
                      <Download className="h-4 w-4 mr-2" />
                      Export Character
                    </>
                  )}
                </Button>
              </div>
            </div>

            <Separator />

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
            </div>
          </CardContent>
        </Card>

        <Card className="pt-6">
          {jobStatus && jobStatus.status !== 'done' && (
            <CardContent className="py-6">
              {jobStatus.status === 'pending' && (
                <p className="text-center">Your request is queued. {jobStatus.position ? `Position: ${jobStatus.position}` : ''}</p>
              )}
              {jobStatus.status === 'processing' && (
                <p className="text-center">Your request is being processed...</p>
              )}
              {jobStatus.status === 'failed' && (
                <p className="text-center text-red-600">{jobStatus.error || 'Job failed'}</p>
              )}
            </CardContent>
          )}
          {jobStatus?.result && (
            <CardHeader className="pb-4 pt-0">
              <CardTitle className="flex items-center gap-2 text-lg">
                {jobStatus.error ? (
                  <>
                    <div className="h-5 w-5 rounded-full bg-red-500" />
                    Export Failed
                  </>
                ) : (
                  <>
                    <div className="h-5 w-5 rounded-full bg-green-500" />
                    Export Successful
                  </>
                )}
              </CardTitle>
            </CardHeader>
          )}
          <CardContent>
            <div className="space-y-4">
              {jobStatus?.result && <div className="flex-col items-center gap-10">
                {jobStatus.result.outputDirectory && <div className="flex items-center gap-2 w-full">
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => {
                      void navigator.clipboard.writeText(jobStatus.result!.outputDirectory!);
                    }}
                    title="Copy output directory"
                  >
                    <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" stroke="currentColor" />
                      <path d="M5 15V5a2 2 0 0 1 2-2h10" stroke="currentColor" />
                    </svg>
                  </Button>
                  <span className="text-lg font-mono select-all">{jobStatus.result!.outputDirectory}</span>
                </div>}
                <div className="flex items-center gap-2 w-full pt-2">
                  <Button variant="default" size="icon" onClick={() => void handleDownloadZip()}>
                    <Download className="h-4 w-4" />
                  </Button>
                  <span className="text-lg">Download: {outputFileName}.zip</span>
                </div>
              </div>}
              {viewerModelPath && (
                <div className="h-[600px]">
                  <ModelViewerUi key={`${viewerModelPath}:${doneCount}`} modelPath={viewerModelPath} />
                </div>
              )}
              {jobStatus?.result && <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <h4 className="font-semibold mb-2">Exported Models:</h4>
                  <ul className="list-disc list-inside space-y-1">
                    {jobStatus.result.exportedModels?.map((model: string, index: number) => (
                      <li key={index} className="text-sm">
                        {jobStatus.result!.versionId ? model.replace(`__${jobStatus.result!.versionId}`, '') : model}
                      </li>
                    ))}
                  </ul>
                </div>
                <div>
                  <h4 className="font-semibold mb-2">Exported Textures:</h4>
                  <ul className="list-disc list-inside space-y-1">
                    {jobStatus.result.exportedTextures?.map((texture: string, index: number) => (
                      <li key={index} className="text-sm">
                        {texture}
                      </li>
                    ))}
                  </ul>
                </div>
              </div>}
            </div>
          </CardContent>
        </Card>
      </div>
      <div className="text-lg text-center text-gray-600 mt-4">
        Created by <a href="https://github.com/pqhuy98" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">wc3-sandbox</a>
        {' | '}
        <a href="https://github.com/pqhuy98/wow-converter" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">Source code</a>
        {' | '}
        <a href="https://www.youtube.com/@wc3-sandbox" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">YouTube</a>
      </div>
    </div>
  );
}

function guessOutputFile(ref: RefSchema) {
  if (ref.type === 'wowhead') {
    return guessOutputFileWowhead(ref.value);
  }
  if (ref.type === 'local') {
    return guessOutputFileLocalPath(ref.value);
  }
  if (ref.type === 'displayID') {
    return guessOutputFileDisplayId(Number(ref.value));
  }
  return undefined;
}

function guessOutputFileWowhead(url: string) {
  // extract npc name from ...npc=1234/name, handling expansion prefixes
  const parts = url.split('#')[0].split('?')[0].split('/');
  // Find the part that contains the category=id/name pattern
  for (let i = parts.length - 1; i >= 0; i--) {
    const part = parts[i];
    if (part.includes('=')) {
      const npcName = (parts[i + 1] || parts[i]).split('=').pop();
      return npcName;
    }
  }
  return undefined;
}

function guessOutputFileLocalPath(path: string) {
  // extract item name from creature\druidcat2\druidcat2_artifact3_green.obj
  return path.split('\\').pop()?.split('.')[0];
}

function guessOutputFileDisplayId(displayId: number) {
  // get npc name from display id
  return `creature-${displayId}`;
}
