import type { Citation } from './api'
// Streaming metadata contains candidates; only references already used in visible text are sources.
export function usedCitations(text:string,citations:Citation[]):Citation[]{
 const used=new Set([...text.matchAll(/\[(\d+)\]/g)].map(v=>Number(v[1])))
 return citations.filter(c=>used.has(c.index))
}
