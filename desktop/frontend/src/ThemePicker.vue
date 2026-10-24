<script setup lang="ts">
import {nextTick,onMounted,onUnmounted,ref} from 'vue'
import {PhSun,PhMoon,PhDesktop,PhCaretDown,PhCheck} from '@phosphor-icons/vue'
import {vGlass} from './glass'
const props=defineProps<{value:string}>()
const emit=defineEmits<{(e:'change',value:string):void}>()
const choices=[{value:'light',name:'亮色',icon:PhSun},{value:'dark',name:'暗色',icon:PhMoon},{value:'system',name:'跟随系统',icon:PhDesktop}]
const root=ref<HTMLElement|null>(null),trigger=ref<HTMLButtonElement|null>(null),opened=ref(false)
function close(restore=false){opened.value=false;if(restore)trigger.value?.focus({preventScroll:true})}
async function open(){opened.value=true;await nextTick();const items=root.value?.querySelectorAll<HTMLButtonElement>('[role=menuitemradio]');items?.[Math.max(0,choices.findIndex(o=>o.value===props.value))]?.focus({preventScroll:true})}
function choose(value:string){emit('change',value);close(true)}
function outside(e:PointerEvent){if(!root.value?.contains(e.target as Node))close()}
function focusOut(e:FocusEvent){if(e.relatedTarget&&!root.value?.contains(e.relatedTarget as Node))close()}
function keys(e:KeyboardEvent){
 if(e.key==='Escape'&&opened.value){e.preventDefault();e.stopPropagation();close(true);return}
 if(!['ArrowDown','ArrowUp','Home','End'].includes(e.key))return
 e.preventDefault();e.stopPropagation()
 if(!opened.value){void open();return}
 const items=Array.from(root.value?.querySelectorAll<HTMLButtonElement>('[role=menuitemradio]')||[]),i=items.indexOf(document.activeElement as HTMLButtonElement)
 items[e.key==='Home'?0:e.key==='End'?items.length-1:(i+(e.key==='ArrowUp'?-1:1)+items.length)%items.length]?.focus({preventScroll:true})
}
const resize=()=>close()
onMounted(()=>{document.addEventListener('pointerdown',outside);window.addEventListener('resize',resize)})
onUnmounted(()=>{document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',resize)})
</script>
<template>
 <div ref="root" class="theme-picker" @keydown="keys" @focusout="focusOut">
  <button ref="trigger" v-glass :title="'主题：'+choices.find(o=>o.value===value)?.name" class="theme-trigger" aria-label="主题" aria-haspopup="menu" :aria-expanded="opened" aria-controls="theme-menu" @click="opened?close(true):open()"><component :is="choices.find(o=>o.value===value)?.icon" :size="18"/><span>{{choices.find(o=>o.value===value)?.name}}</span><PhCaretDown :size="12" :class="{expanded:opened}"/></button>
  <div v-if="opened" id="theme-menu" class="theme-menu" role="menu" aria-label="选择主题"><button v-for="o in choices" :key="o.value" role="menuitemradio" :aria-checked="value===o.value" @click="choose(o.value)"><component :is="o.icon" :size="18"/><span>{{o.name}}</span><PhCheck v-if="value===o.value" :size="16" weight="bold"/></button></div>
 </div>
</template>
<style scoped>
.theme-picker{position:relative;flex-shrink:0}.theme-trigger{height:40px;display:flex;gap:8px;padding:0 13px;border:1px solid var(--line);border-radius:22px;color:var(--accent);font-size:12px;white-space:nowrap}.theme-trigger:hover,.theme-trigger[aria-expanded=true]{background:var(--accent-soft);border-color:var(--accent)}.theme-trigger>svg:last-child{transition:transform .15s}.theme-trigger>svg.expanded{transform:rotate(180deg)}.theme-menu{position:absolute;right:0;top:calc(100% + 9px);z-index:90;min-width:164px;padding:6px;background:var(--surface);border:1px solid var(--line);border-radius:12px;box-shadow:0 10px 32px #0002}.theme-menu button{display:flex;width:100%;justify-content:flex-start;gap:10px;padding:11px 12px;border-radius:7px;font-size:13px;white-space:nowrap;color:var(--text)}.theme-menu button>span{flex:1;text-align:left}.theme-menu button[aria-checked=true]{color:var(--accent);background:var(--accent-soft)}.theme-menu button:hover,.theme-menu button:focus-visible{background:var(--subtle);outline:2px solid var(--accent);outline-offset:-2px}
</style>
