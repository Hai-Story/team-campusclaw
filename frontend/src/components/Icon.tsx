import type { CSSProperties } from 'react'
const paths:Record<string,string>={
 book:'M4 4h6a3 3 0 0 1 2 2v15a4 4 0 0 0-4-2H4z M20 4h-6a3 3 0 0 0-2 2v15a4 4 0 0 1 4-2h4z',
 chat:'M21 11.5a8.5 8.5 0 0 1-8.5 8.5H5l-3 2V11.5a8.5 8.5 0 0 1 17-0',
 agent:'M8 3h8 M12 3v3 M5 7h14v13H5z M8 11h1 M15 11h1 M9 16h6 M2 10v6 M22 10v6',
 spark:'m12 3 2.3 6.7L21 12l-6.7 2.3L12 21l-2.3-6.7L3 12l6.7-2.3z',
 plus:'M12 5v14 M5 12h14', search:'M10.5 3a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15 M16 16l5 5',
 sun:'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1.5 1.5 M17.5 17.5 19 19 M19 5l-1.5 1.5 M6.5 17.5 5 19',
 moon:'M21 13a9 9 0 1 1-10-10 7 7 0 0 0 10 10', logout:'M10 4H4v16h6 M14 7l5 5-5 5 M8 12h11',
 file:'M14 2H5v20h14V7z M14 2v6h5 M8 12h8 M8 16h6', arrow:'M12 19V5 M5 12l7-7 7 7',
 close:'M6 6l12 12 M18 6 6 18', edit:'m16 3 5 5-12 12-6 1 1-6z M14 5l5 5',
 trash:'M3 6h18 M9 6V3h6v3 M6 6l1 15h10l1-15 M10 10v7 M14 10v7',
 download:'M12 3v12 M7 10l5 5 5-5 M4 16v5h16v-5', upload:'M12 16V4 M7 9l5-5 5 5 M4 16v5h16v-5',
 check:'m5 12 4 4 10-10', chevron:'m9 5 7 7-7 7', grid:'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
 link:'m9 15 6-6 M8 17l-2 2a3 3 0 0 1-4-4l5-5a3 3 0 0 1 4 0 M16 7l2-2a3 3 0 0 1 4 4l-5 5a3 3 0 0 1-4 0',
}
export default function Icon({name,size=20,style}:{name:string;size?:number;style?:CSSProperties}){return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={style}><path d={paths[name]??paths.spark}/></svg>}
