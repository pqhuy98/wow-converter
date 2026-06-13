import { spawnSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';

function run(command: string, args: string[]): string | null {
  const result = spawnSync(command, args, {
    encoding: 'utf8',
    windowsHide: false,
    timeout: 5 * 60 * 1000,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) return null;
  const out = result.stdout?.trim();
  return out || null;
}

function psQuote(value: string): string {
  return `'${value.replace(/'/g, "''")}'`;
}

/** Resolve to an existing directory, or its nearest existing parent. */
export function resolveInitialDirectory(initial?: string): string | undefined {
  if (!initial?.trim()) return undefined;

  let candidate = path.resolve(initial.trim());
  for (let i = 0; i < 32; i += 1) {
    try {
      if (fs.statSync(candidate).isDirectory()) return candidate;
    } catch {
      // keep walking up
    }
    const parent = path.dirname(candidate);
    if (parent === candidate) break;
    candidate = parent;
  }
  return undefined;
}

function pickFolderWindows(title: string, initialDirectory?: string): string | null {
  const initial = initialDirectory ? `$dialog.SelectedPath = ${psQuote(initialDirectory)}` : '';
  const script = [
    'Add-Type -AssemblyName System.Windows.Forms',
    '[System.Windows.Forms.Application]::EnableVisualStyles()',
    '$dialog = New-Object System.Windows.Forms.FolderBrowserDialog',
    `$dialog.Description = ${psQuote(title)}`,
    '$dialog.UseDescriptionForTitle = $true',
    '$dialog.ShowNewFolderButton = $false',
    initial,
    'if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {',
    '  Write-Output $dialog.SelectedPath',
    '}',
  ].filter(Boolean).join('; ');
  return run('powershell', ['-NoProfile', '-STA', '-Command', script]);
}

function pickFolderMac(title: string, initialDirectory?: string): string | null {
  const escapedTitle = title.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  const defaultLocation = initialDirectory
    ? ` default location (POSIX file "${initialDirectory.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}")`
    : '';
  const script = `POSIX path of (choose folder with prompt "${escapedTitle}"${defaultLocation})`;
  return run('osascript', ['-e', script]);
}

function pickFolderLinux(title: string, initialDirectory?: string): string | null {
  const zenityArgs = ['--file-selection', '--directory', `--title=${title}`];
  if (initialDirectory) {
    const withSep = initialDirectory.endsWith(path.sep) ? initialDirectory : `${initialDirectory}${path.sep}`;
    zenityArgs.push(`--filename=${withSep}`);
  }
  const zenity = run('zenity', zenityArgs);
  if (zenity !== null) return zenity;

  const startDir = initialDirectory ?? '.';
  return run('kdialog', ['--getexistingdirectory', startDir, '--title', title]);
}

/** Open a native folder picker on the machine running the Express server. */
export function pickNativeFolder(title = 'Select folder', initialDirectory: string | undefined = undefined): string | null {
  const initial = resolveInitialDirectory(initialDirectory);
  const platform = os.platform();
  if (platform === 'win32') return pickFolderWindows(title, initial);
  if (platform === 'darwin') return pickFolderMac(title, initial);
  return pickFolderLinux(title, initial);
}
