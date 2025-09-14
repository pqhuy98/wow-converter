'use client';

import { Download, ExternalLink } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import {
  allAttachments, FullJobStatus,
} from '@/lib/models/export-character.model';
import { formatDurationBetween, formatTimestamp } from '@/lib/utils/format.utils';

import ModelViewerUi from '../../components/common/model-viewer';

export default function RecentsPage() {
  const [jobs, setJobs] = useState<FullJobStatus[]>([]);
  const [expandedJobs, setExpandedJobs] = useState<Set<string>>(new Set());
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [showAbsoluteTime, setShowAbsoluteTime] = useState(false);

  useEffect(() => {
    const fetchRecentJobs = async () => {
      try {
        const response = await fetch('/api/export/character/recent');
        if (!response.ok) {
          throw new Error('Failed to fetch recent jobs');
        }
        const data = await response.json();
        setJobs(data);

        // Set the most recent successful job as selected
        const mostRecentDone = data.find((job: FullJobStatus) => job.status === 'done');
        if (mostRecentDone) {
          setSelectedJobId(mostRecentDone.id);
        }
      } catch (error) {
        console.error('Error fetching recent jobs:', error);
      } finally {
        setLoading(false);
      }
    };

    void fetchRecentJobs();
  }, []);

  const getSimplifiedWowheadUrl = (url: string): string => {
    try {
      const urlObj = new URL(url);
      const pathParts = urlObj.pathname.split('/');
      const categoryPart = pathParts.find((part) => part.includes('='));
      if (categoryPart) {
        const [_category, id] = categoryPart.split('=');
        const slug = pathParts[pathParts.length - 1] || id;
        return `${slug} [${id}]`;
      }
    } catch (e) {
      // If URL parsing fails, return the original value
    }
    return url;
  };

  const getSimplifiedRef = (ref: { type: string; value: string }): string => {
    if (ref.type === 'wowhead') {
      return getSimplifiedWowheadUrl(ref.value);
    } if (ref.type === 'displayID') {
      return `Display ID [${ref.value}]`;
    }
    return ref.value;
  };

  const getAttachItemsString = (attachItems?: Record<string, { path: { type: string; value: string }; scale?: number }>): JSX.Element[] => {
    if (!attachItems || Object.keys(attachItems).length === 0) {
      return [<span key="none">None</span>];
    }

    const attachmentNames = Object.fromEntries(allAttachments.map((a) => [a.id, a.name]));

    return Object.entries(attachItems).map(([attachmentId, item], index) => {
      const attachmentName = attachmentNames[attachmentId] || `Attachment ${attachmentId}`;
      const itemRef = getSimplifiedRef(item.path);

      if (item.path.type === 'wowhead') {
        return (
          <span key={attachmentId}>
            {attachmentName}: {' '}
            <a
              href={item.path.value}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:underline gap-1 inline-flex items-center"
              onClick={(e) => e.stopPropagation()}
            >
              {itemRef}
              <ExternalLink className="h-3 w-3" />
            </a>
            {index < Object.keys(attachItems).length - 1 ? ', ' : ''}
          </span>
        );
      }
      return (
          <span key={attachmentId}>
            {attachmentName}: {itemRef}
            {index < Object.keys(attachItems).length - 1 ? ', ' : ''}
          </span>
      );
    });
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'done': return 'bg-green-500';
      case 'failed': return 'bg-red-500';
      case 'processing': return 'bg-yellow-500';
      case 'pending': return 'bg-gray-500';
      default: return 'bg-gray-500';
    }
  };

  const selectedJob = jobs.find((job) => job.id === selectedJobId);
  const selectedModelPath = useMemo(() => {
    if (selectedJob?.status === 'done' && selectedJob?.result?.exportedModels?.[0]) {
      return selectedJob.result.exportedModels[0];
    }
    return undefined;
  }, [selectedJob?.id, selectedJob?.status, selectedJob?.result?.exportedModels?.[0]]);

  const handleDownloadZip = async (job: FullJobStatus) => {
    if (!job.result) return;

    const files = [
      ...(job.result.exportedModels || []),
      ...(job.result.exportedTextures || []),
    ];

    if (files.length === 0) {
      alert('Nothing to download – exported files list is empty');
      return;
    }

    try {
      const res = await fetch('/api/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ files: files.map(({ path }) => path) }),
      });

      if (!res.ok) {
        throw new Error(await res.text());
      }

      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${job.request.outputFileName}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (e) {
      console.error('Download ZIP error:', e);
      alert(e instanceof Error ? e.message : String(e));
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-4">
        <div className="mx-auto">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto"></div>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full bg-gradient-to-br from-blue-50 to-indigo-100 p-4 flex flex-col overflow-x-hidden">
      <div className="mx-auto flex-1 flex flex-col w-full max-w-full">
        <div className="mb-4" />

        <div className="flex flex-col lg:flex-row gap-6 h-full min-w-0" style={{ height: 'calc(100vh - 125px)' }}>
          {/* Left Column - Job List */}
          <div className="lg:w-1/4 w-full lg:h-full h-[40vh] overflow-hidden min-w-0">
            <Card className="h-full flex flex-col min-w-0">
              <CardHeader className="border-b border-gray-200 py-3">
                <CardTitle className="text-lg">Export History</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 flex-1 overflow-y-auto p-3 min-w-0">
                {jobs.length === 0 ? (
                  <div className="text-center text-gray-500 py-8">
                    No recent exports found
                  </div>
                ) : (
                  jobs.map((job) => {
                    const isExpanded = expandedJobs.has(job.id);
                    const isSelected = selectedJobId === job.id;
                    const character = job.request.character;

                    if (!character) return null;

                    return (
                      <div
                        key={job.id}
                        className={`border rounded-lg p-2 cursor-pointer transition-all duration-200 ${
                          isSelected
                            ? 'border-blue-500 bg-blue-50'
                            : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                        }`}
                        onClick={() => {
                          setSelectedJobId(job.id);
                          // If clicking the same job, toggle its expansion
                          if (selectedJobId === job.id) {
                            setExpandedJobs((prev) => {
                              const newSet = new Set(prev);
                              if (newSet.has(job.id)) {
                                newSet.delete(job.id);
                              } else {
                                newSet.add(job.id);
                              }
                              return newSet;
                            });
                          } else {
                            // If clicking a different job, expand it and collapse all others
                            setExpandedJobs(new Set([job.id]));
                          }
                        }}
                      >
                        {/* Main Row */}
                        <div className="flex items-start justify-between">
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-1">
                              <div className={`w-2 h-2 rounded-full ${getStatusColor(job.status)}`} />
                              <span className="text-sm font-medium text-gray-900 truncate">
                                {job.request.outputFileName}
                              </span>
                            </div>

                            <div className="text-xs text-gray-600 space-y-1">
                              <div className="flex items-start gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Base:</span>
                                <div className="min-w-0 flex-1">
                                  {character.base.type === 'wowhead' ? (
                                    <a
                                      href={character.base.value}
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      className="text-blue-600 hover:underline flex items-center gap-1 font-bold break-all"
                                      onClick={(e) => e.stopPropagation()}
                                    >
                                      {getSimplifiedRef(character.base)}
                                      <ExternalLink className="h-3 w-3 flex-shrink-0" />
                                    </a>
                                  ) : (
                                    <span className="font-bold break-all">{getSimplifiedRef(character.base)}</span>
                                  )}
                                </div>
                              </div>

                              <div className="flex flex-wrap items-center gap-2">
                                <span className="whitespace-nowrap">
                                  <span className="font-medium">Attack:</span> <span className="font-bold">{character.attackTag || 'All'}</span>
                                </span>
                                <span className="whitespace-nowrap">
                                  <span className="font-medium">Size:</span> <span className="font-bold">{character.size || 'Default'}</span>
                                </span>
                              </div>

                              <div className="flex items-start gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Items:</span>
                                <div className="min-w-0 flex-1">
                                  <span className="font-bold break-words">{getAttachItemsString(character.attachItems)}</span>
                                </div>
                              </div>
                              <div className="border-t border-gray-200 mt-2 pt-2">
                                <div className="flex flex-wrap items-center gap-2">
                                  <div className="flex items-center gap-1 min-w-0">
                                    <span className="font-medium whitespace-nowrap">Submitted at: </span>
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      className="text-xs h-auto p-1 font-bold hover:bg-blue-50 border-gray-300 whitespace-nowrap"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        setShowAbsoluteTime(!showAbsoluteTime);
                                      }}
                                    >
                                      {formatTimestamp(job.submittedAt, showAbsoluteTime)}
                                    </Button>
                                  </div>
                                  {job.startedAt && job.finishedAt && job.submittedAt && job.startedAt && (
                                    <div className="flex items-center gap-1 min-w-0">
                                      <span className="font-medium whitespace-nowrap">Duration: </span>
                                      <span className="font-bold whitespace-nowrap">
                                        {formatDurationBetween(job.submittedAt, job.startedAt)} + {formatDurationBetween(job.startedAt, job.finishedAt)}
                                      </span>
                                    </div>
                                  )}
                                </div>
                              </div>
                            </div>
                          </div>
                        </div>

                        {/* Expanded Details */}
                        <div
                          className={`mt-3 pt-3 border-t border-gray-200 text-xs text-gray-600 space-y-2 overflow-hidden transition-all duration-300 ease-in-out ${
                            isExpanded ? 'max-h-96 opacity-100' : 'max-h-0 opacity-0'
                          }`}
                        >
                            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                              <div className="flex items-center gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Walk Speed:</span> <span className="font-bold">{character.inGameMovespeed}</span>
                              </div>
                              <div className="flex items-center gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Scale:</span> <span className="font-bold">{character.scale || '1.0'}</span>
                              </div>
                              <div className="flex items-center gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Format:</span> <span className="font-bold">{job.request.format}</span>
                              </div>
                              <div className="flex items-center gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Model Version:</span> <span className="font-bold">{job.request.formatVersion || '1000'}</span>
                              </div>
                              <div className="flex items-center gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">Keep Cinematic:</span> <span className="font-bold">{character.keepCinematic ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="flex items-center gap-1 min-w-0">
                                <span className="font-medium whitespace-nowrap">No Decay:</span> <span className="font-bold">{character.noDecay ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="flex items-start gap-1 min-w-0 sm:col-span-2">
                                <span className="font-medium whitespace-nowrap">Portrait Camera:</span>
                                <span className="font-bold break-words flex-1">{character.portraitCameraSequenceName || 'None'}</span>
                              </div>
                            </div>

                            {job.request.optimization && (
                              <div className="min-w-0">
                                <span className="font-medium">Optimizations:</span>
                                <div className="mt-1 flex flex-wrap gap-1">
                                  {job.request.optimization.sortSequences && <Badge variant="secondary" className="text-xs">Sort Sequences</Badge>}
                                  {job.request.optimization.removeUnusedVertices && <Badge variant="secondary" className="text-xs">Remove Vertices</Badge>}
                                  {job.request.optimization.removeUnusedNodes && <Badge variant="secondary" className="text-xs">Remove Nodes</Badge>}
                                  {job.request.optimization.removeUnusedMaterialsTextures && <Badge variant="secondary" className="text-xs">Optimize Materials</Badge>}
                                </div>
                              </div>
                            )}

                            {job.status === 'done' && job.result && (
                              <div className="flex items-center gap-2">
                                                                 <Button
                                   variant="outline"
                                   size="sm"
                                   className="text-xs"
                                   onClick={(e) => {
                                     e.stopPropagation();
                                     void handleDownloadZip(job);
                                   }}
                                 >
                                   <Download className="h-3 w-3 mr-1" />
                                   Download
                                 </Button>
                                <span className="text-green-600 font-medium">✓ Complete</span>
                              </div>
                            )}

                            {job.status === 'failed' && (
                              <div className="text-red-600 font-medium">
                                ✗ Failed: {job.error}
                              </div>
                            )}
                          </div>
                        </div>
                    );
                  })
                )}
              </CardContent>
            </Card>
          </div>

          {/* Right Column - Model Viewer */}
          <div className="lg:w-3/4 w-full h-full overflow-hidden min-w-0">
            <div className="p-0 h-full relative overflow-hidden min-w-0">
              <ModelViewerUi modelPath={selectedModelPath?.path} />
              {!selectedModelPath && (
                <div className="absolute inset-0 bg-gray-100 flex items-center justify-center z-10">
                  <div className="text-center text-gray-500">
                    <p className="text-lg mb-2">No model selected</p>
                    <p className="text-sm">Select a completed export from the list to view the model</p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
