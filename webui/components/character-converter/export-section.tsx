import {
  Download, HelpCircle, Loader2, Trash,
} from 'lucide-react';
import { useCallback, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  JobStatus, ModelFormat, ModelFormatVersion, Optimization,
} from '@/lib/models/export-character.model';

import { useServerConfig } from '../server-config';
import { OptimizationOptions } from './optimization-options';

const tooltips = {
  format: 'Model format (MDX vs MDL). MDX is the binary format, the file is most compact and lowest file size. MDL is the text format for debugging purposes, the file is human readable when opened in text editors, but has larger file size.',
  formatVersion: "Model format version (HD vs SD). HD models work in all Warcraft 3 Retail's Reforged and Classic graphics modes, it has the highest fidelity with precise WoW model data. However HD models cannot be opened in legacy modeling tools like Magos Model Editor. If you want to use those legacy tools for post-processing, choose SD 800 instead. WARNING: wow-converter might export broken SD models on complex WoW models. SD conversion does not guarantee to work, after exporting you need to check if each animation is working.",
  delete: 'Delete all exported files inside the "exported-assets" folder',
};

export function ExportSection({
  outputFileName,
  setOutputFileName,
  format,
  setFormat,
  formatVersion,
  setFormatVersion,
  handleExport,
  jobStatus,
  optimization,
  setOptimization,
}: {
  outputFileName: string;
  setOutputFileName: (value: string) => void;
  format: ModelFormat;
  setFormat: (value: ModelFormat) => void;
  formatVersion: ModelFormatVersion;
  setFormatVersion: (value: ModelFormatVersion) => void;
  handleExport: () => Promise<void>;
  jobStatus?: JobStatus;
  optimization: Optimization;
  setOptimization: (value: Optimization) => void;
}) {
  const serverConfig = useServerConfig();

  const [isCleaningAssets, setIsCleaningAssets] = useState(false);
  const cleanAssets = useCallback(() => {
    setIsCleaningAssets(true);
    void fetch('/export/character/clean', { method: 'POST' }).then(() => {
      setIsCleaningAssets(false);
    });
  }, []);

  const isWaiting = jobStatus?.status === 'pending' || jobStatus?.status === 'processing';

  return (
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

          <div className="md:col-span-3 flex items-center gap-1">
            <Button onClick={() => void handleExport()} disabled={isWaiting} className="flex-1" size="lg">
              {isWaiting ? (
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
            {!serverConfig.isSharedHosting && (
              <Button className="px-3 shrink-0" size="lg" variant="destructive"
                onClick={() => cleanAssets()}
                disabled={isCleaningAssets}
              >
                <TooltipProvider delayDuration={0}>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="inline-flex ">
                      {isCleaningAssets
                        ? <Loader2 className="h-4 w-4 animate-spin" />
                        : <Trash className="h-4 w-4" />
                       }
                      </span>
                    </TooltipTrigger>
                    <TooltipContent className="max-w-xs whitespace-normal">
                      <p>{tooltips.delete}</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </Button>
            )}
          </div>
        </div>

        <Separator />

        <OptimizationOptions
          optimization={optimization}
          setOptimization={setOptimization}
        />

      </CardContent>
    </Card>
  );
}
