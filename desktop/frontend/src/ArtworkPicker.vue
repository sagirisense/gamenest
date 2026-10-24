<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {PhMagnifyingGlass,PhSpinnerGap,PhArrowSquareOut} from '@phosphor-icons/vue'
import {call,type Candidate,type ArtworkGame,type ArtworkSettings} from './api'
const props=defineProps<{gameId:string;initialQuery:string;kind:'cover'|'background'}>()
const emit=defineEmits<{(e:'used'):void;(e:'settings'):void;(e:'busy',value:boolean):void}>()
const query=ref(props.initialQuery),games=ref<ArtworkGame[]>([]),chosen=ref(''),images=ref<Candidate[]>([]),pending=ref(false),error=ref(''),message=ref(''),configured=ref(false),loaded=ref(false)
onMounted(async()=>{try{const s=await call<ArtworkSettings>('GetArtworkSettings');configured.value=s.configured;message.value=s.configured?'先查找并确认对应作品，再选择图片。':s.message}catch(e){error.value=String(e)}finally{loaded.value=true}})
async function run(fn:()=>Promise<void>){if(pending.value)return;pending.value=true;emit('busy',true);error.value='';try{await fn()}catch(e){error.value=e instanceof Error?e.message:String(e)}finally{pending.value=false;emit('busy',false)}}
async function loadImages(){images.value=await call<Candidate[]>('GetSteamGridDBArtwork',props.gameId,chosen.value,props.kind)||[];message.value=images.value.length?`找到 ${images.value.length} 张图片，请核对后选择。`:'这个作品暂时没有符合尺寸要求的静态图片。'}
async function search(){await run(async()=>{images.value=[];chosen.value='';games.value=await call<ArtworkGame[]>('SearchArtworkGames',props.gameId,query.value)||[];message.value=games.value.length?'请先选择对应作品。':'未找到作品，可尝试英文原名或 Steam AppID。';if(games.value.length===1){chosen.value=String(games.value[0].id);await loadImages()}})}
async function selectGame(){await run(async()=>{images.value=[];if(chosen.value)await loadImages()})}
async function use(c:Candidate){await run(async()=>{await call('UseArtwork',props.gameId,props.kind,c.key);message.value=props.kind==='cover'?'封面已更新，名称与启动设置保持不变。':'背景图已更新。';emit('used')})}
function view(url:string){if(window.runtime?.BrowserOpenURL)window.runtime.BrowserOpenURL(url);else window.open(url,'_blank','noopener,noreferrer')}
</script>
<template>
 <div class="artwork-picker">
  <p class="artwork-note">SteamGridDB 收录社区图片。优先选择与正式游戏原画一致的版本，自动补全仍优先使用 Steam 正式原画。</p>
  <div v-if="loaded&&!configured" class="artwork-empty"><p>{{message}}</p><button class="secondary small" @click="emit('settings')">前往设置填写 API Key</button></div>
  <template v-else-if="configured">
   <div class="lookup-controls"><input v-model="query" aria-label="SteamGridDB 查询关键词" placeholder="游戏名称 / Steam AppID" :disabled="pending" maxlength="160" @keydown.enter="search"><button class="primary small" :disabled="pending||!query.trim()" @click="search"><PhSpinnerGap v-if="pending" :size="16" class="spinning"/><PhMagnifyingGlass v-else :size="16"/>{{pending?'查询中…':'查找作品'}}</button></div>
   <label v-if="games.length" class="artwork-game-label">对应作品<select v-model="chosen" aria-label="SteamGridDB 对应作品" :disabled="pending" @change="selectGame"><option value="">请选择对应作品</option><option v-for="g in games" :key="g.id" :value="String(g.id)">{{g.name}}</option></select></label>
   <p v-if="message" class="lookup-message" role="status">{{message}}</p>
   <div v-if="images.length" class="artwork-grid" :class="{'background-artworks':kind==='background'}"><article v-for="c in images" :key="c.key"><img :src="c.preview||c.image" :alt="c.name+' '+(kind==='cover'?'封面':'背景')" loading="lazy" referrerpolicy="no-referrer"><div class="artwork-caption"><span>{{c.width}} × {{c.height}}</span><span :title="c.author">{{c.author||'社区上传'}}</span></div><div class="artwork-buttons"><button class="text-button" @click="view(c.url)" :aria-label="'查看图片来源 '+c.key"><PhArrowSquareOut :size="16"/>来源</button><button class="secondary small" :disabled="pending" @click="use(c)">{{kind==='cover'?'使用封面':'使用背景'}}</button></div></article></div>
  </template>
  <p v-if="error" class="inline-error" role="alert">{{error}}</p>
 </div>
</template>
