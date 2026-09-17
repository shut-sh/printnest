export function previewURL(file: File): string {
  return URL.createObjectURL(file);
}

export function revokeURL(url: string): void {
  URL.revokeObjectURL(url);
}

export function isPDF(file: File): boolean {
  return file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf');
}

export function isImage(file: File): boolean {
  if (file.type.startsWith('image/')) return true;
  return /\.(jpe?g|png|webp|heic|heif)$/i.test(file.name);
}
