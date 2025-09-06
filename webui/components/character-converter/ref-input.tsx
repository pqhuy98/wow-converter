import { AlertCircle, CheckCircle, HelpCircle } from 'lucide-react';
import { useCallback, useState } from 'react';

import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import { RefSchema, RefType } from '@/lib/models/export-character.model';

import { useServerConfig } from '../server-config';

export type RefCategory = 'npc' | 'item'

export function RefInput({
  value,
  onChange,
  label,
  tooltip,
  category,
}: {
  value: RefSchema
  onChange: (ref: RefSchema) => void
  label: string
  category: RefCategory
  tooltip: string
}) {
  const { isSharedHosting } = useServerConfig();
  const [currentValues, setCurrentValues] = useState<Record<RefType, RefSchema>>({
    wowhead: value.type === 'wowhead' ? value : { type: 'wowhead', value: '' },
    local: value.type === 'local' ? value : { type: 'local', value: '' },
    displayID: value.type === 'displayID' ? value : { type: 'displayID', value: '' },
  });

  const [clientValidationResult, setClientValidationResult] = useState<{ ok: boolean, error: string | null } | null>(null);
  const [serverValidationResult, setServerValidationResult] = useState<{ ok: boolean, similarFiles: string[], error: string | null } | null>(null);

  const clientValidation = useCallback((value: RefSchema, fix: boolean) => {
    const error = validateRef(value, category, fix);
    if (fix) {
      setCurrentValues((prev) => ({ ...prev, [value.type]: value }));
      onChange(value);
    }
    if (error) {
      setClientValidationResult({ ok: false, error });
      return false;
    }
    setClientValidationResult({ ok: true, error: null });
    return true;
  }, []);

  const fullValidAndFix = useCallback((value: RefSchema) => {
    if (!clientValidation(value, true)) return;
    if (value.type === 'local') {
      void fetch(`/export/character/check-local-file?localPath=${value.value}`)
        .then((res) => res.json())
        .then((data) => setServerValidationResult(data));
    } else {
      setServerValidationResult({ ok: true, similarFiles: [], error: null });
    }
  }, []);

  const localFileNotFound = serverValidationResult && value.type === 'local' && !serverValidationResult.ok;
  const otherSkins = serverValidationResult?.similarFiles.filter((file) => file !== currentValues.local.value) ?? [];
  const hasOtherSkins = serverValidationResult && value.type === 'local'
    && otherSkins.length > 0
    && otherSkins.every((file) => file.startsWith(currentValues.local.value.replaceAll('.obj', '').replaceAll('/', '\\')));

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Label className="text-sm font-medium">{label}</Label>
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger>
              <HelpCircle className="h-4 w-4 text-muted-foreground" />
            </TooltipTrigger>
            <TooltipContent>
              <p className="max-w-xs">{tooltip}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
        <Select value={value.type} onValueChange={(type: RefType) => {
          fullValidAndFix({ ...currentValues[type], type });
        }}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="start">
            <SelectItem value="wowhead">Wowhead URL</SelectItem>
            {!isSharedHosting ? (
              <SelectItem value="local">Local File</SelectItem>
            ) : null}
            <SelectItem value="displayID">Display ID</SelectItem>
          </SelectContent>
        </Select>

        <div className="md:col-span-2">
          <Input
            placeholder={
              value.type === 'local'
                ? 'Enter relative OBJ file name...'
                : value.type === 'wowhead'
                  ? `https://www.wowhead.com/${category}=12345/...`
                  : 'Enter Display ID number...'
            }
            value={currentValues[value.type].value}
            onChange={(e) => {
              const newValue = { ...currentValues[value.type], value: e.target.value };
              setCurrentValues((prev) => ({ ...prev, [value.type]: newValue }));
              clientValidation(newValue, false);
            }}
            // detect paste
            onPaste={(e) => {
              const newValue = { ...currentValues[value.type], value: e.clipboardData.getData('text').trim() };
              e.preventDefault();
              e.stopPropagation();
              fullValidAndFix(newValue);
            }}
            onBlur={() => {
              fullValidAndFix(currentValues[value.type]);
            }}
            onFocus={(e) => {
              e.target.select();
            }}
            className={`border-2 bg-white text-left ${clientValidationResult && !clientValidationResult.ok ? 'border-red-500' : 'border-gray-300 focus:border-blue-500'}`}
          />
          {clientValidationResult && clientValidationResult.error && (
            <div className="flex items-center gap-1 mt-1 text-sm text-red-600">
              <AlertCircle className="h-3 w-3" />
              {clientValidationResult.error}
            </div>
          )}

          {localFileNotFound ? (
            <div className="flex items-center gap-1 mt-1 text-sm text-red-600">
              <AlertCircle className="h-3 w-3" />
              Local file not found
            </div>
          ) : null}

          {hasOtherSkins
            ? (<div className="flex flex-col  ">
                <div className={`flex items-center gap-1 mt-1 text-sm ${localFileNotFound ? 'text-red-600' : 'text-green-600'}`}>
                  {!localFileNotFound && <CheckCircle className="h-3 w-3" />}
                  {localFileNotFound ? 'Do you mean:' : 'Other related skins:'}
                </div>
                <ul className="list-disc list-outside text-left mx-auto max-w-md text-sm">
                  {serverValidationResult.similarFiles.filter((file) => file !== currentValues.local.value).map((file) => <li key={file}
                      onClick={() => {
                        const newValue = { ...value, value: file.replaceAll('"', '') };
                        setCurrentValues((prev) => ({ ...prev, [value.type]: newValue }));
                        onChange(newValue);
                        clientValidation(newValue, true);
                        fullValidAndFix(newValue);
                      }}
                      className="text-blue-500 cursor-pointer hover:underline">
                        {file}
                    </li>)}
                </ul>
              </div>)
            : null
          }
        </div>
      </div>
    </div>
  );
}

const wowheadPattern = {
  npc: /^https:\/\/www\.wowhead\.com\/(?:[a-z-]+\/)?(npc=|item=|object=|dressing-room(\?.+)?#)/,
  item: /^https:\/\/www\.wowhead\.com\/(?:[a-z-]+\/)?item=/,
};

const invalidMessage = {
  npc: 'Invalid Wowhead URL, must contain either: "/npc=", "/item=", "/object=" or "/dressing-room#"',
  item: 'Invalid Wowhead URL, must contain /item=...',
};

export const validateRef = (ref: RefSchema, category: RefCategory, fix: boolean): string | null => {
  if (ref.type === 'wowhead') {
    // Allow URLs with expansion prefixes like /wotlk/, /classic/, etc. (only a-z characters)
    if (!wowheadPattern[category].test(ref.value)) {
      return invalidMessage[category];
    }
  }
  if (ref.type === 'local') {
    if (fix) {
      if (ref.value.startsWith('"')) ref.value = ref.value.slice(1);
      if (ref.value.endsWith('"')) ref.value = ref.value.slice(0, -1);
      const serverConfig = useServerConfig();
      if (serverConfig?.wowExportAssetDir && ref.value.startsWith(serverConfig.wowExportAssetDir)) {
        ref.value = ref.value.slice(serverConfig.wowExportAssetDir.length);
      }
      if (ref.value.startsWith('/') || ref.value.startsWith('\\')) ref.value = ref.value.slice(1);
      if (/\[[0-9]+\]/.test(ref.value)) ref.value = ref.value.replace(/\[[0-9]+\]/, '');
      if (ref.value.endsWith('.m2')) ref.value = ref.value.replace('.m2', '.obj');
      if (ref.value.endsWith('.wmo')) ref.value = ref.value.replace('.wmo', '.obj');
      if (!ref.value.endsWith('.obj')) ref.value += '.obj';
      if (ref.value.includes('/')) ref.value = ref.value.replaceAll('/', '\\');
    }
    if (!isLocalRef(ref.value)) {
      return 'Invalid local file path';
    }
  }
  if (ref.type === 'displayID' && (isNaN(Number(ref.value)) || Number(ref.value) <= 0)) {
    return 'Invalid Display ID, must be a number';
  }
  return null;
};

const localRefPattern = /^[a-zA-Z0-9_\-/\\,]+(\.obj)?$/;

export function isLocalRef(val: string) {
  if (!localRefPattern.test(val)) return false;
  // Must not be absolute path
  // Must not contain ".." as a path segment
  if (val.split('/').some((seg) => seg === '..')) return false;
  // Must not start with "/" or "\"
  if (val.startsWith('/') || val.startsWith('\\')) return false;
  // Must not contain null bytes or suspicious chars
  if (val.includes('\0')) return false;
  return true;
}
