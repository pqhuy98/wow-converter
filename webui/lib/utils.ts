import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function isLocalRef(val: string) {
  // Must not be absolute path
  // Must not contain ".." as a path segment
  if (val.split(/[\\/]/).some((seg) => seg === '..')) return false;
  // Must not start with "/" or "\"
  if (/^[\\/]/.test(val)) return false;
  // Must not contain null bytes or suspicious chars
  if (/[\0]/.test(val)) return false;
  return true;
}