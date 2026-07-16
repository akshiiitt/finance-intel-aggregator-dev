import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function sanitizeUrl(url: string | undefined | null): string {
  if (!url) return "#"
  const trimmed = url.trim()
  if (trimmed.toLowerCase().startsWith("http://") || trimmed.toLowerCase().startsWith("https://")) {
    return trimmed
  }
  return "#"
}
