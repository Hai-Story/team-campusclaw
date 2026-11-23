// The version endpoint is public metadata; never send session credentials to it.
export async function availableBuild(current:string,signal:AbortSignal,fetcher:typeof fetch=fetch):Promise<string|null>{
 const response=await fetcher('/build-info.json',{cache:'no-store',credentials:'omit',signal})
 if(!response.ok)return null
 const value:unknown=await response.json()
 if(typeof value!=='object'||value===null||!('build' in value)||typeof value.build!=='string'||!/^[a-f0-9]{16}$/.test(value.build))return null
 return value.build===current?null:value.build
}
