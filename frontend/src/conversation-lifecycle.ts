// An already started read can settle after a successful DELETE. Never restore it.
export async function restoreIfPresent<T>(id:number,load:(id:number)=>Promise<T>,isDeleted:(id:number)=>boolean,restore:(value:T)=>void):Promise<boolean> {
 if(isDeleted(id))return false
 const value=await load(id)
 if(isDeleted(id))return false
 restore(value)
 return true
}
