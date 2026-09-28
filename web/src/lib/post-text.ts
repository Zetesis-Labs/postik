// The same rules as internal/core/posts: strip the editor's HTML, decode its
// entities and count UTF-16 code units, so the editor and the server agree.
const limits: Record<string, number> = {
  telegram: 4096,
  linkedin: 3000,
  'linkedin-page': 3000,
};

export function characterLimit(provider: string): number {
  return limits[provider] ?? 0;
}

export function plainText(content: string): string {
  const stripped = content.replace(/<[^>]*>/g, '');
  const decoder = document.createElement('textarea');
  decoder.innerHTML = stripped;
  return decoder.value;
}

export function countCharacters(text: string): number {
  return text.length;
}
