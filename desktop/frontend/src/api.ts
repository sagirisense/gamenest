export interface Candidate {score?:number;key:string;name:string;image?:string;url:string;provider:string;exact:boolean;preview?:string;author?:string;width?:number;height?:number}
export interface ArtworkGame {id:number;name:string}
export interface ArtworkSettings {configured:boolean;message:string}
export interface Background {provider?:string;status:string;message:string;image?:string;appid?:string;url?:string;source_url?:string;width?:number;height?:number;candidates?:Candidate[]}
export interface Game {id:string;name:string;original_name:string;source:string;exe:string;folder:string;executables:string[];image:string;hero_image:string;background:Background;has_cover:boolean;detected_appid:string;favorite:boolean;last_played:string;metadata:Record<string,any>|null;review?:{pending:boolean;reason:string;count:number}}
export interface Library {games:Game[];theme:string;updated:string;data_dir:string;note:string}
declare global {interface Window {go?:{main:{App:Record<string,(...args:any[])=>Promise<any>>}};runtime?:Record<string,(...args:any[])=>any>}}
let token=''
export const isDesktop=()=>!!window.go?.main?.App
export async function library():Promise<Library>{if(isDesktop())return window.go!.main.App.GetLibrary();const r=await fetch('/api/library');if(!r.ok)throw Error(await r.text());const d=await r.json();token=d.token;return d.library}
export async function call<T=void>(method:string,...args:string[]):Promise<T>{if(isDesktop())return window.go!.main.App[method](...args);const r=await fetch('/api/call/'+method,{method:'POST',headers:{'Content-Type':'application/json','X-Library-Token':token},body:JSON.stringify(args)});const d=await r.json();if(!r.ok)throw Error(d.error||'操作失败');return d}
export function windowAction(action:string){window.runtime?.[action]?.()}
