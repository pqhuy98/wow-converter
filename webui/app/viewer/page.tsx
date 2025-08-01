"use client"

import { ArrowLeft } from "lucide-react"
import { useSearchParams, useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import ModelViewerUi from "../model-viewer"

export default function ViewerPage() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const modelPath = searchParams.get('model')

  if (!modelPath) {
    return (
      <div className="h-screen w-screen bg-black flex items-center justify-center">
        <div className="text-white text-center">
          <h1 className="text-2xl mb-4">No model specified</h1>
          <p className="mb-4">Please provide a model path via the 'model' query parameter.</p>
          <Button
            variant="secondary"
            onClick={() => router.back()}
            className="bg-white/10 text-white border-white/20 hover:bg-white/20"
          >
            <ArrowLeft className="h-4 w-4 mr-2" />
            Go Back
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="h-screen w-screen bg-black relative">
      <div className="absolute top-4 left-4 z-10">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => router.back()}
          className="bg-black/50 text-white border-white/20 hover:bg-black/70"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back
        </Button>
      </div>
      <ModelViewerUi modelPath={decodeURIComponent(modelPath)} alwaysFullscreen={true} />
    </div>
  )
} 