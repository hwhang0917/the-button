<script setup lang="ts">
import { ref } from 'vue'
import type { Card } from '../useGame'
import { t } from '../i18n'
import { play, vibrate } from '../audio'
import { burst, confetti } from '../particles'
import { COIN_COLORS } from '../useCoinCounter'
import CardReveal from './CardReveal.vue'

const props = defineProps<{ card: Card }>()
const emit = defineEmits<{ close: [] }>()

// sealed -> tearing (brief burst animation) -> revealed (CardReveal takes over)
const stage = ref<'sealed' | 'tearing' | 'revealed'>('sealed')

const RARITY_FX: Record<string, { colors: string[]; count: number; sound: 'success_silver' | 'success_gold' | 'success_diamond' | 'win' }> = {
  common: { colors: COIN_COLORS, count: 40, sound: 'success_silver' },
  rare: { colors: ['#7dd3fc', '#38bdf8', '#0ea5e9', '#ffffff'], count: 70, sound: 'success_gold' },
  holo: { colors: ['#f0abfc', '#fcd34d', '#22d3ee', '#a78bfa', '#ffffff'], count: 110, sound: 'success_diamond' },
  prismatic: { colors: ['#ff0084', '#fcff00', '#00fff0', '#7c00ff', '#ffffff'], count: 160, sound: 'win' },
}
const TEAR_MS = 450

function tear() {
  if (stage.value !== 'sealed') return
  stage.value = 'tearing'
  const fx = RARITY_FX[props.card.rarity]
  burst(window.innerWidth / 2, window.innerHeight / 2, fx.colors, fx.count)
  play(fx.sound)
  vibrate(props.card.rarity === 'prismatic' ? [40, 30, 80, 30, 120] : [20, 25, 45])
  if (props.card.rarity === 'prismatic') confetti()
  setTimeout(() => (stage.value = 'revealed'), TEAR_MS)
}
</script>

<template>
  <CardReveal v-if="stage === 'revealed'" :card="card" @close="emit('close')" />
  <div
    v-else
    class="fixed inset-0 z-50 flex flex-col items-center justify-center gap-6 bg-black/80 backdrop-blur-sm"
  >
    <p class="text-sm tracking-widest text-slate-400">{{ t('packTear') }}</p>
    <button
      class="pack-seal relative flex h-72 w-52 flex-col items-center justify-center gap-3 rounded-2xl border-2 border-amber-300/60 bg-gradient-to-b from-indigo-800 via-violet-900 to-slate-900"
      :class="stage === 'tearing' ? 'pack-tear' : 'pack-idle cursor-pointer'"
      @click="tear"
    >
      <img src="/star.png" class="h-20 w-20 drop-shadow-[0_0_25px_#facc15]" alt="" />
      <p class="text-xl font-black tracking-widest text-amber-300">THE BUTTON</p>
      <p class="text-[10px] uppercase tracking-[0.3em] text-indigo-200">{{ t('packName') }}</p>
      <span class="absolute inset-x-4 top-3 border-t-2 border-dashed border-amber-200/40"></span>
    </button>
  </div>
</template>

<style scoped>
@keyframes pack-idle {
  0%, 100% { transform: rotate(-1.5deg) scale(1); box-shadow: 0 0 30px #a78bfa66; }
  50% { transform: rotate(1.5deg) scale(1.03); box-shadow: 0 0 55px #a78bfaaa; }
}
.pack-idle {
  animation: pack-idle 1.6s ease-in-out infinite;
}
@keyframes pack-tear {
  0% { transform: scale(1); opacity: 1; filter: brightness(1); }
  40% { transform: scale(1.18); filter: brightness(2.2); }
  100% { transform: scale(1.6); opacity: 0; filter: brightness(3); }
}
.pack-tear {
  animation: pack-tear 0.45s ease-out forwards;
  pointer-events: none;
}
</style>
