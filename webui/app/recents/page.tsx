"use client"

import { useState, useEffect, useMemo } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ExternalLink, Download, ArrowLeft, History } from "lucide-react"
import { host } from "../config"
import ModelViewerUi from "../model-viewer"
import { commonAttachments, FullJobStatus, JobStatus, otherAttachments } from "@/lib/models/export-character.model"

// Utility function to format timestamps
const formatTimestamp = (timestamp: number, showAbsolute: boolean = false): string => {
  if (showAbsolute) {
    const date = new Date(timestamp)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    const seconds = String(date.getSeconds()).padStart(2, '0')
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  }
  
  const now = Date.now()
  const diff = now - timestamp
  const minutes = Math.floor(diff / (1000 * 60))
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  
  if (days > 0) {
    return `${days} day${days > 1 ? 's' : ''} ago`
  } else if (hours > 0) {
    return `${hours} hour${hours > 1 ? 's' : ''} ago`
  } else if (minutes > 0) {
    return `${minutes} min${minutes > 1 ? 's' : ''} ago`
  } else {
    return 'Just now'
  }
}

// Utility function to calculate duration
const calculateDuration = (startTime?: number, endTime?: number): string | null => {
  if (!startTime || !endTime) return null
  
  const duration = endTime - startTime
  const seconds = Math.floor(duration / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  
  if (minutes > 0) {
    return `${minutes}m ${remainingSeconds}s`
  } else {
    return `${seconds}s`
  }
}

export default function RecentsPage() {
  const [jobs, setJobs] = useState<FullJobStatus[]>([])
  const [expandedJobs, setExpandedJobs] = useState<Set<string>>(new Set())
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [showAbsoluteTime, setShowAbsoluteTime] = useState(false)

  useEffect(() => {
    const fetchRecentJobs = async () => {
      try {
        const response = await fetch(`${host}/export/character/recent`)
        if (!response.ok) {
          throw new Error('Failed to fetch recent jobs')
        }
        const data = await response.json()
        setJobs(data)
        
        // Set the most recent successful job as selected
        const mostRecentDone = data.find((job: FullJobStatus) => job.status === 'done')
        if (mostRecentDone) {
          setSelectedJobId(mostRecentDone.id)
        }
      } catch (error) {
        console.error('Error fetching recent jobs:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchRecentJobs()
  }, [])

  const toggleExpanded = (jobId: string) => {
    const newExpanded = new Set(expandedJobs)
    if (newExpanded.has(jobId)) {
      newExpanded.delete(jobId)
    } else {
      newExpanded.add(jobId)
    }
    setExpandedJobs(newExpanded)
  }

  const getSimplifiedWowheadUrl = (url: string, type: 'npc' | 'item'): string => {
    try {
      const urlObj = new URL(url)
      const pathParts = urlObj.pathname.split('/')
      const categoryPart = pathParts.find(part => part.includes('='))
      if (categoryPart) {
        const [category, id] = categoryPart.split('=')
        const slug = pathParts[pathParts.length - 1] || id
        return `${slug} [${id}]`
      }
    } catch (e) {
      // If URL parsing fails, return the original value
    }
    return url
  }

  const getSimplifiedRef = (ref: { type: string; value: string }, type: 'npc' | 'item'): string => {
    if (ref.type === 'wowhead') {
      return getSimplifiedWowheadUrl(ref.value, type)
    } else if (ref.type === 'displayID') {
      return `Display ID [${ref.value}]`
    } else {
      return ref.value
    }
  }

  const getAttachItemsString = (attachItems?: Record<string, { path: { type: string; value: string }; scale?: number }>): JSX.Element[] => {
    if (!attachItems || Object.keys(attachItems).length === 0) {
      return [<span key="none">None</span>]
    }

    const attachmentNames = Object.fromEntries([...commonAttachments, ...otherAttachments].map(a => [a.id, a.name]))

    return Object.entries(attachItems).map(([attachmentId, item], index) => {
      const attachmentName = attachmentNames[attachmentId] || `Attachment ${attachmentId}`
      const itemRef = getSimplifiedRef(item.path, 'item')
      
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
        )
      } else {
        return (
          <span key={attachmentId}>
            {attachmentName}: {itemRef}
            {index < Object.keys(attachItems).length - 1 ? ', ' : ''}
          </span>
        )
      }
    })
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'done': return 'bg-green-500'
      case 'failed': return 'bg-red-500'
      case 'processing': return 'bg-yellow-500'
      case 'pending': return 'bg-gray-500'
      default: return 'bg-gray-500'
    }
  }

  const selectedJob = jobs.find(job => job.id === selectedJobId)
  const selectedModelPath = useMemo(() => {
    if (selectedJob?.status === 'done' && selectedJob?.result?.exportedModels?.[0]) {
      return `${selectedJob.result.exportedModels[0]}?v=${selectedJob.id}`
    }
    return undefined
  }, [selectedJob?.id, selectedJob?.status, selectedJob?.result?.exportedModels?.[0]])

  const handleDownloadZip = async (job: FullJobStatus) => {
    if (!job.result) return

    const files = [
      ...(job.result.exportedModels || []),
      ...(job.result.exportedTextures || []),
    ]

    if (files.length === 0) {
      alert("Nothing to download – exported files list is empty")
      return
    }

    try {
      const res = await fetch(`${host}/download`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ files }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const blob = await res.blob()
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = `${job.request.outputFileName}.zip`
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
    } catch (e: any) {
      console.error("Download ZIP error:", e)
      alert(e?.message || String(e))
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-4">
        <div className="max-w-7xl mx-auto">
          <div className="text-center">
            <h1 className="text-4xl font-bold text-gray-900 mb-4">Recent Exports</h1>
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto"></div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-4 flex flex-col">
      <div className="max-w-7xl mx-auto flex-1 flex flex-col">
        <div className="flex justify-between items-center mb-6">
          <div className="text-center flex-1">
            <h1 className="text-4xl font-bold text-gray-900">Recent Exports</h1>
            <p className="text-lg text-gray-600 mt-2">View recent character exports</p>
          </div>
          <Button
            variant="outline"
            onClick={() => window.location.href = '/'}
            className="flex items-center gap-2"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Exporter
          </Button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6" style={{ height: 'calc(100vh - 125px)' }}>
          {/* Left Column - Job List */}
          <div className="lg:col-span-1 h-full overflow-hidden">
            <Card className="h-full flex flex-col">
              <CardHeader className="border-b border-gray-200">
                <CardTitle className="text-lg">Export History</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 flex-1 overflow-y-auto p-3">
                {jobs.length === 0 ? (
                  <div className="text-center text-gray-500 py-8">
                    No recent exports found
                  </div>
                ) : (
                  jobs.map((job) => {
                    const isExpanded = expandedJobs.has(job.id)
                    const isSelected = selectedJobId === job.id
                    const character = job.request.character
                    
                    if (!character) return null

                    return (
                      <div
                        key={job.id}
                        className={`border rounded-lg p-3 cursor-pointer transition-all duration-200 ${
                          isSelected 
                            ? 'border-blue-500 bg-blue-50' 
                            : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                        }`}
                        onClick={() => {
                          setSelectedJobId(job.id)
                          // If clicking the same job, toggle its expansion
                          if (selectedJobId === job.id) {
                            setExpandedJobs(prev => {
                              const newSet = new Set(prev)
                              if (newSet.has(job.id)) {
                                newSet.delete(job.id)
                              } else {
                                newSet.add(job.id)
                              }
                              return newSet
                            })
                          } else {
                            // If clicking a different job, expand it and collapse all others
                            setExpandedJobs(new Set([job.id]))
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
                              <div className="flex items-center gap-1">
                                <span className="font-medium">Base:</span>
                                {character.base.type === 'wowhead' ? (
                                  <a
                                    href={character.base.value}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="text-blue-600 hover:underline flex items-center gap-1 font-bold"
                                    onClick={(e) => e.stopPropagation()}
                                  >
                                    {getSimplifiedRef(character.base, 'npc')}
                                    <ExternalLink className="h-3 w-3" />
                                  </a>
                                ) : (
                                  <span className="font-bold">{getSimplifiedRef(character.base, 'npc')}</span>
                                )}
                              </div>
                              
                              <div className="flex items-center gap-4">
                                <span>
                                  <span className="font-medium">Attack:</span> <span className="font-bold">{character.attackTag || 'All'}</span>
                                </span>
                                <span>
                                  <span className="font-medium">Size:</span> <span className="font-bold">{character.size || 'Default'}</span>
                                </span>
                              </div>
                              
                              <div>
                                <span className="font-medium">Items:</span> <span className="font-bold">{getAttachItemsString(character.attachItems)}</span>
                              </div>
                              <div className="border-t border-gray-200 mt-2 pt-2">
                                <div className="flex items-center gap-2">
                                  <div className="w-48 flex items-center gap-1">
                                    <span className="font-medium whitespace-nowrap">Submitted at: </span> 
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      className="text-xs h-auto p-1 font-bold hover:bg-blue-50 border-gray-300"
                                      onClick={(e) => {
                                        e.stopPropagation()
                                        setShowAbsoluteTime(!showAbsoluteTime)
                                      }}
                                    >
                                      {formatTimestamp(job.submittedAt, showAbsoluteTime)}
                                    </Button>
                                  </div>
                                  {job.startedAt && job.finishedAt && job.submittedAt && job.startedAt && (
                                    <div className="w-32 flex items-center gap-1">
                                      <span className="font-medium">Duration: </span> 
                                      <span className="font-bold">
                                        {calculateDuration(job.submittedAt, job.startedAt)} + {calculateDuration(job.startedAt, job.finishedAt)}
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
                            <div className="grid grid-cols-2 gap-2">
                              <div className="w-48 flex items-center gap-1">
                                <span className="font-medium">Walk Speed:</span> <span className="font-bold">{character.inGameMovespeed}</span>
                              </div>
                              <div className="w-48 flex items-center gap-1">
                                <span className="font-medium">Scale:</span> <span className="font-bold">{character.scale || '1.0'}</span>
                              </div>
                              <div className="w-48 flex items-center gap-1">
                                <span className="font-medium">Format:</span> <span className="font-bold">{job.request.format}</span>
                              </div>
                              <div className="w-48 flex items-center gap-1">
                                <span className="font-medium">Keep Cinematic:</span> <span className="font-bold">{character.keepCinematic ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="w-48 flex items-center gap-1">
                                <span className="font-medium">No Decay:</span> <span className="font-bold">{character.noDecay ? 'Yes' : 'No'}</span>
                              </div>
                              <div className="w-48 flex items-center gap-1">
                                <span className="font-medium">Portrait Camera:</span> <span className="font-bold">{character.portraitCameraSequenceName || 'None'}</span>
                              </div>
                            </div>
                            

                            
                            {job.request.optimization && (
                              <div>
                                <span className="font-medium">Optimizations:</span>
                                <div className="mt-1 space-y-1">
                                  {job.request.optimization.sortSequences && <Badge variant="secondary" className="text-xs mr-1">Sort Sequences</Badge>}
                                  {job.request.optimization.removeUnusedVertices && <Badge variant="secondary" className="text-xs mr-1">Remove Vertices</Badge>}
                                  {job.request.optimization.removeUnusedNodes && <Badge variant="secondary" className="text-xs mr-1">Remove Nodes</Badge>}
                                  {job.request.optimization.removeUnusedMaterialsTextures && <Badge variant="secondary" className="text-xs mr-1">Optimize Materials</Badge>}
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
                                     e.stopPropagation()
                                     handleDownloadZip(job)
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
                    )
                  })
                )}
              </CardContent>
            </Card>
          </div>

          {/* Right Column - Model Viewer */}
          <div className="lg:col-span-2 h-full overflow-hidden">
            <div className="p-0 h-full relative overflow-hidden">
              <ModelViewerUi modelPath={selectedModelPath} />
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
  )
} 