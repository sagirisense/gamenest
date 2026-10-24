<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {call} from './api'
defineProps<{disabled:boolean}>()
const emit=defineEmits<{(e:'busy',value:boolean):void}>()
const roots=ref(''),excluded=ref(''),message=ref(''),failed=ref(false),loaded=ref(false)
type Settings={roots:string[];excluded:string[]}
function show(s:Settings){roots.value=s.roots.join('\n');excluded.value=s.excluded.join('\n')}
onMounted(async()=>{try{show(await call<Settings>('GetScanSettings'));loaded.value=true}catch(e){failed.value=true;message.value=String(e)}})
async function action(fn:()=>Promise<void>){emit('busy',true);message.value='';failed.value=false;try{await fn()}catch(e){failed.value=true;message.value=String(e)}finally{emit('busy',false)}}
async function browse(target:'roots'|'excluded'){await action(async()=>{const path=await call<string>('PickScanFolder');if(path){const field=target==='roots'?roots:excluded;field.value=[field.value.trim(),path].filter(Boolean).join('\n')}})}
async function save(){await action(async()=>{const split=(s:string)=>s.split(/\r?\n/).map(p=>p.trim()).filter(Boolean);show(await call<Settings>('SaveScanSettings',JSON.stringify({roots:split(roots.value),excluded:split(excluded.value)})));message.value='已保存，下次扫描生效。'})}
</script>
<template>
 <section class="scan-settings"><h3>扫描范围</h3><p>每行一个完整路径，例如 D:\ 或 E:\Games。排除位置及其子文件夹不会扫描。</p>
 <label>扫描位置<textarea v-model="roots" aria-label="扫描位置" rows="3" placeholder="D:\&#10;E:\Games" :disabled="disabled||!loaded" spellcheck="false"/></label>
 <button class="secondary small" :disabled="disabled||!loaded" @click="browse('roots')">添加扫描文件夹</button>
 <label class="scan-excluded">排除位置<textarea v-model="excluded" aria-label="排除位置" rows="3" placeholder="D:\备份&#10;D:\工具" :disabled="disabled||!loaded" spellcheck="false"/></label>
 <div class="api-commands"><button class="secondary small" :disabled="disabled||!loaded" @click="browse('excluded')">添加排除文件夹</button><button class="primary small" :disabled="disabled||!loaded||!roots.trim()" @click="save">保存扫描范围</button></div>
 <p>保存后点击“扫描游戏”更新清单。范围外和被排除的游戏会移出清单；自定义名称、封面和收藏仍保留，重新纳入扫描后可恢复。</p>
 <p v-if="message" :class="{'inline-error':failed}" role="status">{{message}}</p>
 </section>
</template>
