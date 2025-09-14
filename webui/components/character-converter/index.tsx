'use client';

import { downloadAssetsZip } from '@api/download';
import _ from 'lodash';
import { Download } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';

import { AttachItems } from '@/components/character-converter/attach-items';
import { CharacterConfig } from '@/components/character-converter/character-config';
import { ExportSection } from '@/components/character-converter/export-section';
import { isLocalRef, validateRef } from '@/components/character-converter/ref-input';
import ModelViewerUi from '@/components/common/model-viewer';
import { Terminal } from '@/components/common/terminal';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import {
  allAttachments,
  AttachItem, Character, ExportRequest, JobStatus, ModelFormat, ModelFormatVersion, Optimization, RefSchema,
} from '@/lib/models/export-character.model';
import { formatFileSize } from '@/lib/utils/format.utils';

import { useServerConfig } from '../server-config';

const defaultCharacter = {
  base: { type: 'wowhead', value: 'https://www.wowhead.com/wotlk/npc=36597/the-lich-king' },
  size: 'hero',
  attackTag: 'Auto',
  inGameMovespeed: 270,
  attachItems: {
    1: {
      path: { type: 'wowhead', value: 'https://www.wowhead.com/classic/item=231885/frostmourne' },
    },
  },
  portraitCameraSequenceName: 'Stand',
} as const;

export function CharacterConverter() {
  const serverConfig = useServerConfig();
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
  const [optimization, setOptimization] = useState<Optimization>({
    sortSequences: true,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true,
    maxTextureSize: serverConfig.isClassic ? '512' : undefined,
  });

  useEffect(() => {
    if (serverConfig.isClassic && !optimization.maxTextureSize) {
      setOptimization({ ...optimization, maxTextureSize: '512' });
    }
  }, [serverConfig.isClassic]);

  // Job/queue tracking
  const [jobStatus, setJobStatus] = useState<JobStatus | undefined>(undefined);
  const [viewerModelPath, setViewerModelPath] = useState<string | undefined>(undefined);

  useEffect(() => {
    const checkExportResult = async () => {
      const res = await fetch('/api/export/character/demos');
      const jobs = (await res.json()) as JobStatus[];
      if (jobs.length > 0) {
        setViewerModelPath(jobs[0].result?.exportedModels[0].path);
      }
    };
    void checkExportResult();
  }, []);

  const addAttachItem = () => {
    // Find the first unused attachment ID, starting with common ones
    const usedIds = new Set(Object.keys(character.attachItems || {}).map(Number));

    let newId = allAttachments[0].id;
    for (const attachment of allAttachments) {
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
    if (!outputFileName.trim()) return 'Output file name is required';
    if (!isLocalRef(outputFileName)) return 'Invalid output file name';

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
    setJobStatus({
      id: '',
      status: 'pending',
      submittedAt: Date.now(),
      logs: [],
    });

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

      const response = await fetch('/api/export/character', {
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
        error: error instanceof Error ? error.message : String(error),
        submittedAt: Date.now(),
        logs: [],
      });
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
    if (!jobStatus || jobStatus.status === 'done' || jobStatus.status === 'failed' || !jobStatus.id) return undefined;

    let pendingFetches = 0;
    const fetchJobStatus = async () => {
      try {
        pendingFetches++;
        if (pendingFetches > 1) return;
        const res = await fetch(`/api/export/character/status/${jobStatus.id}`);
        if (!res.ok) {
          throw new Error(await res.text());
        }
        const data = (await res.json()) as JobStatus;
        setJobStatus(data);

        if (data.status === 'pending') {
          // do nothing
        } else if (data.status === 'processing') {
          // do nothing
        } else if (data.status === 'done' && data.result) {
          setDoneCount(doneCount + 1);
          setViewerModelPath(data.result.exportedModels[0].path);
          clearInterval(interval);
        } else if (data.status === 'failed') {
          clearInterval(interval);
        }
      } catch (e: unknown) {
        console.error('Polling error:', e);
        setJobStatus({
          id: '',
          status: 'failed',
          error: e instanceof Error ? e.message : String(e),
          submittedAt: Date.now(),
          logs: [],
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

    await downloadAssetsZip({ files: files.map(({ path }) => path), source: 'export' });
  };

  return (
    <div className="min-h-[calc(100vh-57px)] p-4">
      <div className="max-w-6xl mx-auto space-y-4">

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <CharacterConfig
            character={character}
            setCharacter={setCharacter}
            clearOutputFileName={() => setOutputFileName('')}
          />
          <AttachItems
            character={character}
            setCharacter={setCharacter}
            removeAttachItem={removeAttachItem}
            updateAttachItem={updateAttachItem}
            addAttachItem={addAttachItem}
          />
        </div>

        <ExportSection
          outputFileName={outputFileName}
          setOutputFileName={setOutputFileName}
          format={format}
          setFormat={setFormat}
          formatVersion={formatVersion}
          setFormatVersion={setFormatVersion}
          handleExport={handleExport}
          jobStatus={jobStatus}
          optimization={optimization}
          setOptimization={setOptimization}
        />

        <Card className="pt-6">
          {jobStatus && jobStatus.status !== 'done' && (
            <CardContent className="mb-2">
              {(jobStatus.status === 'processing' || jobStatus.status === 'pending') && (
                <>
                  <div className="flex items-center justify-center gap-2">
                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500" />
                    <p className="text-lg">{jobStatus.status === 'processing' ? 'Exporting...' : `Queue position: ${jobStatus.position}`}</p>
                  </div>
                  <Terminal className='mt-4' logs={jobStatus.logs || []} />
                </>
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
                  <h4 className="font-semibold mb-2">Model Stats:</h4>
                  {[
                    { label: 'Format Version', value: jobStatus.result!.modelStats.formatVersion },
                    { label: 'Global Sequences', value: jobStatus.result!.modelStats.globalSequences },
                    { label: 'Sequences', value: jobStatus.result!.modelStats.sequences },
                    { label: 'Bones', value: jobStatus.result!.modelStats.bones },
                    { label: 'Geosets', value: jobStatus.result!.modelStats.geosets },
                    { label: 'Geoset Anims', value: jobStatus.result!.modelStats.geosetAnims },
                    { label: 'Vertices', value: jobStatus.result!.modelStats.vertices },
                    { label: 'Faces', value: jobStatus.result!.modelStats.faces },
                    { label: 'Textures', value: jobStatus.result!.modelStats.textures },
                    { label: 'Texture Anims', value: jobStatus.result!.modelStats.textureAnims },
                    { label: 'Materials', value: jobStatus.result!.modelStats.materials },
                    { label: 'Lights', value: jobStatus.result!.modelStats.lights },
                    { label: 'Ribbon Emitters', value: jobStatus.result!.modelStats.ribbonEmitters },
                    { label: 'Particle Emitters', value: jobStatus.result!.modelStats.particles },
                    { label: 'Attachments', value: jobStatus.result!.modelStats.attachments },
                    { label: 'Event Objects', value: jobStatus.result!.modelStats.eventObjects },
                    { label: 'Helpers', value: jobStatus.result!.modelStats.helpers },
                    { label: 'Collision Shapes', value: jobStatus.result!.modelStats.collisionShapes },
                    { label: 'Cameras', value: jobStatus.result!.modelStats.cameras },
                  ].map(({ label, value }) => (
                    <div key={label} className="text-sm break-all">
                      {label}: {value}
                    </div>
                  ))}
                </div>
                <div>
                  <h4 className="font-semibold mb-2">
                  Exported files:
                  <span className="text-muted-foreground font-normal ml-4">{
                    (() => {
                      const modelsSize = jobStatus.result.exportedModels?.reduce((acc, { size }) => acc + size, 0) || 0;
                      const texturesSize = jobStatus.result.exportedTextures?.reduce((acc, { size }) => acc + size, 0) || 0;
                      const totalSize = modelsSize + texturesSize;
                      return formatFileSize(totalSize);
                    })()
                  }
                  </span>
                  </h4>
                  <ul className="list-disc list-outside pl-6 space-y-1">
                    {jobStatus.result.exportedModels?.map(({ path, size }, index: number) => (
                      <li key={index} className="text-sm break-all">
                        <b><span>
                          {jobStatus.result!.versionId ? path.replace(`__${jobStatus.result!.versionId}`, '') : path}
                        </span></b>
                        <span className="select-none ml-4"></span>
                        <span className="text-muted-foreground">{formatFileSize(size)}</span>                      </li>
                    ))}
                    {jobStatus.result.exportedTextures?.map(({ path, size }, index: number) => (
                      <li key={index} className="text-sm break-all">
                        <span> {path}</span>
                        <span className="select-none ml-4"></span>
                        <span className="text-muted-foreground">{formatFileSize(size)}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>}
            </div>
          </CardContent>
        </Card>
      </div>
      <div className="text-lg text-center text-muted-foreground mt-4">
        Created by <a href="https://github.com/pqhuy98" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">wc3-sandbox</a>
        {' | '}
        <a href="https://github.com/pqhuy98/wow-converter" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">Source code</a>
        {' | '}
        <a href="https://www.youtube.com/@wc3-sandbox" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">YouTube</a>
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
