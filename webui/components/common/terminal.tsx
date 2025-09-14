import React from 'react';

// ANSI color codes mapping
const ANSI_COLORS = {
  // Reset
  0: 'reset',
  // Colors
  30: 'black',
  31: 'red',
  32: 'green',
  33: 'yellow',
  34: 'blue',
  35: 'magenta',
  36: 'cyan',
  37: 'white',
  // Bright colors
  90: 'brightBlack',
  91: 'brightRed',
  92: 'brightGreen',
  93: 'brightYellow',
  94: 'brightBlue',
  95: 'brightMagenta',
  96: 'brightCyan',
  97: 'brightWhite',
  // Background colors
  40: 'bgBlack',
  41: 'bgRed',
  42: 'bgGreen',
  43: 'bgYellow',
  44: 'bgBlue',
  45: 'bgMagenta',
  46: 'bgCyan',
  47: 'bgWhite',
  // Bright background colors
  100: 'bgBrightBlack',
  101: 'bgBrightRed',
  102: 'bgBrightGreen',
  103: 'bgBrightYellow',
  104: 'bgBrightBlue',
  105: 'bgBrightMagenta',
  106: 'bgBrightCyan',
  107: 'bgBrightWhite',
} as const;

// CSS color mapping
const CSS_COLORS = {
  black: 'text-foreground',
  red: 'text-red-600',
  green: 'text-green-600',
  yellow: 'text-yellow-600',
  blue: 'text-blue-600',
  magenta: 'text-purple-600',
  cyan: 'text-cyan-600',
  white: 'text-foreground',
  brightBlack: 'text-muted-foreground',
  brightRed: 'text-red-500',
  brightGreen: 'text-green-500',
  brightYellow: 'text-yellow-500',
  brightBlue: 'text-blue-500',
  brightMagenta: 'text-purple-500',
  brightCyan: 'text-cyan-500',
  brightWhite: 'text-foreground',
  bgBlack: 'bg-secondary',
  bgRed: 'bg-red-600',
  bgGreen: 'bg-green-600',
  bgYellow: 'bg-yellow-600',
  bgBlue: 'bg-blue-600',
  bgMagenta: 'bg-purple-600',
  bgCyan: 'bg-cyan-600',
  bgWhite: 'bg-card',
  bgBrightBlack: 'bg-secondary',
  bgBrightRed: 'bg-red-500',
  bgBrightGreen: 'bg-green-500',
  bgBrightYellow: 'bg-yellow-500',
  bgBrightBlue: 'bg-blue-500',
  bgBrightMagenta: 'bg-purple-500',
  bgBrightCyan: 'bg-cyan-500',
  bgBrightWhite: 'bg-card',
} as const;

interface ParsedSegment {
  text: string;
  color?: string;
  backgroundColor?: string;
  bold?: boolean;
  dim?: boolean;
  italic?: boolean;
  underline?: boolean;
}

function parseAnsiString(input: string): ParsedSegment[] {
  const segments: ParsedSegment[] = [];
  let currentSegment: ParsedSegment = { text: '' };
  let i = 0;

  while (i < input.length) {
    if (input[i] === '\x1b' && input[i + 1] === '[') {
      // Found ANSI escape sequence
      let j = i + 2;
      let code = '';

      // Extract the code
      while (j < input.length && input[j] !== 'm') {
        code += input[j];
        j++;
      }

      if (j < input.length && input[j] === 'm') {
        // Parse the code
        const codes = code.split(';').filter((c) => c !== '');

        // Check if this is a reset command
        if (codes.includes('0')) {
          // Save current segment if it has text
          if (currentSegment.text) {
            segments.push({ ...currentSegment });
          }
          // Reset all formatting
          currentSegment = { text: '' };
        } else {
          // Save current segment if it has text and we're changing formatting
          if (currentSegment.text) {
            segments.push({ ...currentSegment });
          }

          // Create new segment with new formatting
          currentSegment = { text: '' };
          // Apply formatting to new segment
          for (const c of codes) {
            if (c === '1') {
              currentSegment.bold = true;
            } else if (c === '2') {
              currentSegment.dim = true;
            } else if (c === '3') {
              currentSegment.italic = true;
            } else if (c === '4') {
              currentSegment.underline = true;
            } else if (ANSI_COLORS[Number(c) as keyof typeof ANSI_COLORS]) {
              const colorName = ANSI_COLORS[Number(c) as keyof typeof ANSI_COLORS];
              if (colorName.startsWith('bg')) {
                currentSegment.backgroundColor = CSS_COLORS[colorName as keyof typeof CSS_COLORS];
              } else {
                currentSegment.color = CSS_COLORS[colorName as keyof typeof CSS_COLORS];
              }
            }
          }
        }

        i = j + 1;
        continue;
      }
    }

    currentSegment.text += input[i];
    i++;
  }

  // Add the last segment if it has text
  if (currentSegment.text) {
    segments.push(currentSegment);
  }

  return segments;
}

interface TerminalProps {
  logs: string[];
  className?: string;
}

export function Terminal({ logs, className = '' }: TerminalProps) {
  return (
    <div className={`bg-secondary text-foreground font-mono text-sm p-4 rounded-lg border border-border max-h-40 overflow-y-auto ${className}`}>
      <div className="space-y-1">
        {logs.map((log, index) => {
          const segments = parseAnsiString(log);
          return (
            <div key={index} className="flex overflow-hidden whitespace-nowrap">
              {segments.map((segment, segIndex) => {
                const classes = [
                  segment.color || 'text-foreground',
                  segment.backgroundColor,
                  segment.bold && 'font-bold',
                  segment.dim && 'opacity-50',
                  segment.italic && 'italic',
                  segment.underline && 'underline',
                ].filter(Boolean).join(' ');

                return (
                  <span key={segIndex} className={classes}>
                    {segment.text}
                  </span>
                );
              })}
            </div>
          );
        })}
      </div>
    </div>
  );
}
