<script setup lang="ts">
import {PhWarningCircle,PhArrowSquareOut,PhGameController} from '@phosphor-icons/vue'
import type {Game,Candidate} from './api'
import {vGlass} from './glass'
defineProps<{game:Game;candidates:Candidate[];selected:string;disabled:boolean}>()
const emit=defineEmits<{(e:'select',key:string):void;(e:'open',url:string):void}>()
function score(c:Candidate){return c.score!==undefined?`${Math.min(100,Math.round(c.score*100))}%`:c.exact?'编号或名称一致':'待核对'}
</script>
<template>
 <section class="review-content">
  <div v-glass class="review-explanation"><PhWarningCircle :size="24" weight="fill"/><div><strong>{{game.review?.reason||'请核对对应作品'}}</strong><p>请对照封面、名称和作品编号选择。确认前保留当前资料。</p></div></div>
  <div class="review-original"><img v-if="game.image" :src="game.image" alt="当前封面"><div><small>当前资料</small><h3>{{game.name}}</h3><p>{{game.original_name}}</p><code :title="game.folder">{{game.folder}}</code></div></div>
  <h3 class="review-heading">选择对应作品 <span>{{candidates.length}} 个候选</span></h3>
  <div role="radiogroup" aria-label="游戏资料候选" class="review-options">
   <article v-for="c in candidates" :key="c.key" v-glass class="review-option" :class="{chosen:selected===c.key}">
    <label><input type="radio" name="review-candidate" :value="c.key" :checked="selected===c.key" :disabled="disabled" @change="emit('select',c.key)"><img v-if="c.image" :src="c.image" :alt="c.name" referrerpolicy="no-referrer"><PhGameController v-else :size="44"/><div><h4>{{c.name}}</h4><p>{{c.provider}}</p><code>{{c.key}}</code><span class="candidate-score">{{c.score!==undefined?'匹配率 ':''}}{{score(c)}}</span></div></label>
    <button class="text-button" :disabled="disabled" @click="emit('open',c.url)">查看商店 <PhArrowSquareOut :size="14"/></button>
   </article>
  </div>
 </section>
</template>
