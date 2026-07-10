const MAX_REGEX_PATTERN_LEN = 256;

export function safeRegexPattern(pattern: string): string | null {
  if (!pattern || pattern.length > MAX_REGEX_PATTERN_LEN) return null;
  if (nestedQuantifierPattern(pattern)) return null;
  try {
    // Compile check only — actual search uses the validated pattern.
    // eslint-disable-next-line no-new
    new RegExp(pattern, 'i');
    return pattern;
  } catch {
    return null;
  }
}

function nestedQuantifierPattern(pattern: string): boolean {
  for (let i = 0; i < pattern.length - 2; i++) {
    if (pattern[i] !== '(') continue;
    let depth = 1;
    for (let j = i + 1; j < pattern.length; j++) {
      if (pattern[j] === '(') depth++;
      else if (pattern[j] === ')') {
        depth--;
        if (depth === 0) {
          const inner = pattern.slice(i + 1, j);
          if (/[+*/]/.test(inner) && j + 1 < pattern.length && /[+*}]/.test(pattern.slice(j + 1))) {
            return true;
          }
          break;
        }
      }
    }
  }
  return false;
}
