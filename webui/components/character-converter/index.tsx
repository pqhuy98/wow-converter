'use client';

import { downloadAssetsZip } from '@api/download';
import _ from 'lodash';
import { Download } from 'lucide-react';
import {
  useCallback, useEffect, useRef, useState,
} from 'react';

import { ReportForm } from '@/components/bug-reports/report-form';
import { AttachItems } from '@/components/character-converter/attach-items';
import { CharacterConfig } from '@/components/character-converter/character-config';
import { ExportSection } from '@/components/character-converter/export-section';
import { isLocalRef, validateRef } from '@/components/character-converter/ref-input';
import { SequenceEditButton, SequencePanel } from '@/components/character-converter/sequence-panel';
import ModelViewerUi from '@/components/common/model-viewer/model-viewer';
import { Terminal } from '@/components/common/terminal';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import {
  allAttachments,
  AttachItem, Character, defaultTextureBakingForm, ExportRequest, JobStatus, ModelFormat, ModelFormatVersion, Optimization, RefSchema,
  toTextureBakingRequest,
} from '@/lib/models/export-character.model';

import { useServerConfig } from '../server-config';

function formatFileSize(size: number): string {
  const MAX_B = 1024;
  const MAX_KB = MAX_B * MAX_B;
  const MAX_MB = MAX_KB * MAX_B;

  if (size < MAX_B) {
    return `${size}B`;
  }
  if (size < MAX_KB) {
    return `${(size / MAX_B).toFixed(2)}KB`;
  }
  if (size < MAX_MB) {
    return `${(size / MAX_KB).toFixed(2)}MB`;
  }
  return `${(size / MAX_MB).toFixed(2)}GB`;
}

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
    allMaterialsUnshaded: false,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true,
    maxTextureSize: serverConfig.isClassic ? '512' : undefined,
  });
  const [textureBaking, setTextureBaking] = useState(defaultTextureBakingForm);

  useEffect(() => {
    if (serverConfig.isClassic && !optimization.maxTextureSize) {
      setOptimization({ ...optimization, maxTextureSize: '512' });
    }
  }, [serverConfig.isClassic]);

  // Job/queue tracking
  const [jobStatus, setJobStatus] = useState<JobStatus | undefined>(undefined);
  const [progressInViewer, setProgressInViewer] = useState(false);
  const [completedJob, setCompletedJob] = useState<JobStatus>();
  const [sequenceRequest, setSequenceRequest] = useState<ExportRequest>();
  const [playbackSelection, setPlaybackSelection] = useState<{ name: string; revision: number }>();
  const sourceKey = JSON.stringify([serverConfig.buildKey, character.base]);
  const previousSource = useRef(sourceKey);
  useEffect(() => {
    if (previousSource.current !== sourceKey) {
      previousSource.current = sourceKey;
      setCharacter((previous) => ({ ...previous, sequenceSelection: undefined }));
      setSequenceRequest(undefined);
    }
  }, [sourceKey]);
  const [viewerModelPath, setViewerModelPath] = useState<string | undefined>(undefined);
  const [reportSequence, setReportSequence] = useState(0);

  useEffect(() => {
    const checkExportResult = async () => {
      const res = await fetch('/api/export/character/demos');
      const jobs = (await res.json()) as JobStatus[];
      if (jobs.length > 0) {
        setViewerModelPath(jobs[0].result?.exportedModels?.[0]?.path);
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

  const makeExportRequest = (): ExportRequest => ({
    character: { ...character, attackTag: character.attackTag ?? '', sequenceSelection: undefined },
    outputFileName,
    optimization,
    textureBaking: toTextureBakingRequest(textureBaking),
    format,
    formatVersion,
  });

  const handleExport = async (override?: ExportRequest): Promise<boolean> => {
    const error = override ? null : checkExportValid();
    if (error) {
      alert(error);
      return false;
    }
    setProgressInViewer(!!override);
    if (!override) setSequenceRequest(undefined);
    setJobStatus({
      id: '',
      status: 'pending',
      submittedAt: Date.now(),
      logs: [],
    });

    try {
      // Prepare request
      const request = override ?? makeExportRequest();

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

      const result: JobStatus = await response.json();
      if (result.status === 'failed') throw new Error(result.error ?? 'Export failed');
      setJobStatus(result);
      if (result.status === 'done' && result.result) {
        setCompletedJob(result);
        setPlaybackSelection(undefined);
        setViewerModelPath(result.result.exportedModels?.[0]?.path);
        setDoneCount((previous) => previous + 1);
      }
      return true;
    } catch (error: unknown) {
      console.error('Export error:', error);
      setJobStatus({
        id: '',
        status: 'failed',
        error: error instanceof Error ? error.message : String(error),
        submittedAt: Date.now(),
        logs: [],
      });
      return false;
    }
  };

  // is window focused?
  const [isWindowFocused, setIsWindowFocused] = useState(true);
  useEffect(() => {
    const handleFocus = () => setIsWindowFocused(true);
    const handleBlur = () => setIsWindowFocused(false);
    window.addEventListener('focus', handleFocus);
    window.addEventListener('blur', handleBlur);
    return () => {
      window.removeEventListener('focus', handleFocus);
      window.removeEventListener('blur', handleBlur);
    };
  }, []);

  const [doneCount, setDoneCount] = useState(0);

  // Poll job status every 1s when a job is active
  useEffect(() => {
    if (!jobStatus || jobStatus.status === 'done' || jobStatus.status === 'failed' || !jobStatus.id) return undefined;

    let cancelled = false;
    let fetching = false;
    const fetchJobStatus = async () => {
      if (fetching) return;
      fetching = true;
      try {
        const res = await fetch(`/api/export/character/status/${jobStatus.id}`);
        if (!res.ok) {
          throw new Error(await res.text());
        }
        const data = (await res.json()) as JobStatus;
        if (cancelled) return;
        setJobStatus(data);

        if (data.status === 'pending') {
          // do nothing
        } else if (data.status === 'processing') {
          // do nothing
        } else if (data.status === 'done' && data.result) {
          setCompletedJob(data);
          setPlaybackSelection(undefined);
          setDoneCount((previous) => previous + 1);
          setViewerModelPath(data.result.exportedModels?.[0]?.path);
          clearInterval(interval);
        } else if (data.status === 'failed') {
          clearInterval(interval);
        }
      } catch (e: unknown) {
        if (cancelled) return;
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
        fetching = false;
      }
    };

    const interval = setInterval(() => void fetchJobStatus(), 1000);
    void fetchJobStatus();

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [jobStatus?.id, isWindowFocused]);

  /**
   * Download the exported assets as a ZIP by calling the new POST /download API.
   */
  const handleDownloadZip = async () => {
    if (!completedJob?.result) return;

    const files = [
      ...(completedJob.result.exportedModels || []),
      ...(completedJob.result.exportedTextures || []),
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
          handleExport={async () => { await handleExport(); }}
          jobStatus={jobStatus}
          optimization={optimization}
          setOptimization={setOptimization}
          textureBaking={textureBaking}
          setTextureBaking={setTextureBaking}
        />

        <Card className="pt-6">
          {(!progressInViewer || !viewerModelPath) && jobStatus && jobStatus.status !== 'done' && (
            <CardContent className="mb-2">
              {(jobStatus.status === 'processing' || jobStatus.status === 'pending') && (
                <>
                  <div className="flex items-center justify-center gap-2">
                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary" />
                    <p className="text-lg">{jobStatus.status === 'processing' ? 'Exporting...' : `Queue position: ${jobStatus.position}`}</p>
                  </div>
                  <Terminal className='mt-4' logs={jobStatus.logs || []} />
                </>
              )}
              {jobStatus.status === 'failed' && (
                <p className="text-center text-destructive">{jobStatus.error || 'Job failed'}</p>
              )}
            </CardContent>
          )}
          {completedJob?.result && (
            <CardHeader className="pb-4 pt-0">
              <CardTitle className="flex items-center gap-2 text-lg">
                {completedJob.error ? (
                  <>
                    <div className="h-5 w-5 rounded-full bg-red-500" />
                    Export Failed
                  </>
                ) : (
                  <>
                    <div className="h-5 w-5 rounded-full bg-green-500" />
                    Export Successful
                    {completedJob.finishedAt && completedJob.startedAt && (
                      <span className="text-muted-foreground ml-1">
                        ({((completedJob.finishedAt - completedJob.startedAt) / 1000).toFixed(2)}s)
                      </span>
                    )}
                  </>
                )}
              </CardTitle>
            </CardHeader>
          )}
          <CardContent>
            <div className="space-y-4">
              {completedJob?.result && <div className="flex-col items-center gap-10">
                {completedJob.result.outputDirectory && <div className="flex items-center gap-2 w-full">
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => {
                      void navigator.clipboard.writeText(completedJob.result!.outputDirectory!);
                    }}
                    title="Copy output directory"
                  >
                    <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" stroke="currentColor" />
                      <path d="M5 15V5a2 2 0 0 1 2-2h10" stroke="currentColor" />
                    </svg>
                  </Button>
                  <span className="text-lg font-mono select-all">{completedJob.result!.outputDirectory}</span>
                </div>}
                <div className="flex items-center gap-2 w-full pt-2">
                  <Button variant="default" size="icon" onClick={() => void handleDownloadZip()}>
                    <Download className="h-4 w-4" />
                  </Button>
                  <span className="text-lg">Download: {completedJob.result.reportMetadata?.request.outputFileName ?? outputFileName}.zip</span>
                </div>
              </div>}
              <div className="flex flex-col lg:flex-row gap-3">
              {viewerModelPath && (
                <div className="h-[600px] border-border flex-1 min-w-0 relative">
                  <ModelViewerUi modelPath={viewerModelPath} modelRevision={doneCount} onSequenceChange={setReportSequence} playbackSelection={playbackSelection}
                    animationHeaderAction={completedJob?.result?.reportMetadata && completedJob.result.reportMetadata.buildKey === serverConfig.buildKey && !completedJob.result.reportMetadata.request.character.mount?.path.value && completedJob.result.reportMetadata.models.find((model) => model.path === viewerModelPath)?.sequences.some((sequence) => sequence.sourceIndices?.length) && <SequenceEditButton editing={!!sequenceRequest} busy={jobStatus?.status === 'pending' || jobStatus?.status === 'processing'} onClick={() => setSequenceRequest(sequenceRequest ? undefined : completedJob.result!.reportMetadata!.request)} />}
                    animationList={sequenceRequest && <SequencePanel
                character={sequenceRequest.character}
                buildKey={serverConfig.buildKey}
                skinId={sequenceRequest.skinId}
                busy={jobStatus?.status === 'pending' || jobStatus?.status === 'processing'}
                playbackSequences={samePreviewContext(sequenceRequest, completedJob?.result?.reportMetadata?.request) ? completedJob?.result?.reportMetadata?.models.find((model) => model.path === viewerModelPath)?.sequences ?? [] : []}
                onPreview={(name) => setPlaybackSelection((previous) => ({ name, revision: (previous?.revision ?? 0) + 1 }))}
                onApply={async (selection) => {
                  const request = { ...sequenceRequest, modelScale: completedJob?.result?.reportMetadata?.models.find((model) => model.path === viewerModelPath)?.modelScale, character: { ...sequenceRequest.character, sequenceSelection: selection } };
                  if (!await handleExport(request)) return false;
                  previousSource.current = JSON.stringify([serverConfig.buildKey, request.character.base]);
                  setCharacter({ ...request.character, sequenceSelection: undefined });
                  setOutputFileName(request.outputFileName);
                  setOptimization(request.optimization);
                  setFormat(request.format ?? 'mdx');
                  setFormatVersion(request.formatVersion ?? '1000');
                  setTextureBaking({ ...defaultTextureBakingForm, ...request.textureBaking, enabled: request.textureBaking?.enabled ?? false });
                  return true;
                }}
              />}
                    canvasOverlay={progressInViewer && jobStatus && jobStatus.status !== 'done' && <div className="absolute inset-0 z-20 flex items-center justify-center px-3 pointer-events-none">
                    <div className={`pointer-events-auto mx-auto w-full max-w-xl rounded-lg border bg-background/95 p-4 shadow-lg backdrop-blur-sm ${jobStatus.status === 'failed' ? 'border-destructive/50' : 'border-border'}`}>
                      {jobStatus.status === 'failed' ? <>
                        <p className="text-sm font-medium text-destructive">Export failed</p>
                        <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded border border-border bg-card p-2 text-xs text-destructive">{jobStatus.error || 'Job failed'}</pre>
                      </> : <>
                        <div className="flex items-center gap-2">
                          <div className="h-4 w-4 animate-spin rounded-full border-b-2 border-primary" />
                          <p className="text-sm font-medium">{!jobStatus.id ? 'Starting export…' : jobStatus.status === 'processing' ? 'Exporting…' : `Queue position: ${jobStatus.position ?? '…'}`}</p>
                        </div>
                        {(jobStatus.logs?.length ?? 0) > 0 && <Terminal logs={jobStatus.logs ?? []} className="mt-3 max-h-48 w-full" />}
                      </>}
                    </div>
                  </div>}
                  />

                </div>
              )}
              </div>
              {completedJob?.result && <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {!serverConfig.isSharedHosting && completedJob.status === 'done' && viewerModelPath && completedJob.result.reportMetadata?.request.character.base.type === 'wowhead' && <div className="md:col-span-2">
                  <ReportForm key={completedJob.id} exportId={completedJob.id} metadata={completedJob.result.reportMetadata} modelPath={viewerModelPath} currentSequence={reportSequence} />
                </div>}
                <div>
                  <h4 className="font-semibold mb-2">Model Stats:</h4>
                  {[
                    { label: 'Format Version', value: completedJob.result!.modelStats.formatVersion },
                    { label: 'Global Sequences', value: completedJob.result!.modelStats.globalSequences },
                    { label: 'Sequences', value: completedJob.result!.modelStats.sequences },
                    { label: 'Bones', value: completedJob.result!.modelStats.bones },
                    { label: 'Geosets', value: completedJob.result!.modelStats.geosets },
                    { label: 'Geoset Anims', value: completedJob.result!.modelStats.geosetAnims },
                    { label: 'Vertices', value: completedJob.result!.modelStats.vertices },
                    { label: 'Faces', value: completedJob.result!.modelStats.faces },
                    { label: 'Textures', value: completedJob.result!.modelStats.textures },
                    { label: 'Texture Anims', value: completedJob.result!.modelStats.textureAnims },
                    { label: 'Materials', value: completedJob.result!.modelStats.materials },
                    { label: 'Lights', value: completedJob.result!.modelStats.lights },
                    { label: 'Ribbon Emitters', value: completedJob.result!.modelStats.ribbonEmitters },
                    { label: 'Particle Emitters', value: completedJob.result!.modelStats.particles },
                    { label: 'Attachments', value: completedJob.result!.modelStats.attachments },
                    { label: 'Event Objects', value: completedJob.result!.modelStats.eventObjects },
                    { label: 'Helpers', value: completedJob.result!.modelStats.helpers },
                    { label: 'Collision Shapes', value: completedJob.result!.modelStats.collisionShapes },
                    { label: 'Cameras', value: completedJob.result!.modelStats.cameras },
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
                      const modelsSize = completedJob.result.exportedModels?.reduce((acc, { size }) => acc + size, 0) || 0;
                      const texturesSize = completedJob.result.exportedTextures?.reduce((acc, { size }) => acc + size, 0) || 0;
                      const totalSize = modelsSize + texturesSize;
                      return formatFileSize(totalSize);
                    })()
                  }
                  </span>
                  </h4>
                  <ul className="list-disc list-outside pl-6 space-y-1">
                    {completedJob.result.exportedModels?.map(({ path, size }, index: number) => (
                      <li key={index} className="text-sm break-all">
                        <b><span>
                          {completedJob.result!.versionId ? path.replace(`__${completedJob.result!.versionId}`, '') : path}
                        </span></b>
                        <span className="select-none ml-4"></span>
                        <span className="text-muted-foreground">{formatFileSize(size)}</span>                      </li>
                    ))}
                    {completedJob.result.exportedTextures?.map(({ path, size }, index: number) => (
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

function samePreviewContext(request: ExportRequest, completed?: ExportRequest): boolean {
  if (!completed) return false;
  const { sequenceSelection: _selection, ...character } = request.character;
  const { sequenceSelection: _completedSelection, ...completedCharacter } = completed.character;
  return _.isEqual(character, completedCharacter) && (request.skinId ?? '') === (completed.skinId ?? '');
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
  // extract item name from creature\druidcat2\druidcat2_artifact3_green
  return path.split('\\').pop()?.split('.')[0];
}

function guessOutputFileDisplayId(displayId: number) {
  // get npc name from display id
  return `creature-${displayId}`;
}
