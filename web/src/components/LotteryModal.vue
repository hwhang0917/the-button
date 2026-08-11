<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { LOTTERY_PRIZES, state } from '../useGame'
import { t } from '../i18n'
import { play, scratchTick, vibrate } from '../audio'
import { burst, confetti } from '../particles'
import { COIN_COLORS } from '../useCoinCounter'

const props = defineProps<{ prize: number; finalCoins: number }>()
defineEmits<{ close: [] }>()

const CW = 256
const CH = 112
const REVEAL_RATIO = 0.55

const cv = ref<HTMLCanvasElement | null>(null)
const revealed = ref(false)
let scratching = false
let strokes = 0

const tierLabel = computed(() => {
  const idx = LOTTERY_PRIZES.indexOf(props.prize)
  return idx >= 0 ? t('lotteryTiers')[idx] : ''
})

onMounted(() => {
  const ctx = cv.value!.getContext('2d')!
  const g = ctx.createLinearGradient(0, 0, CW, CH)
  g.addColorStop(0, '#cbd5e1')
  g.addColorStop(0.5, '#94a3b8')
  g.addColorStop(1, '#cbd5e1')
  ctx.fillStyle = g
  ctx.fillRect(0, 0, CW, CH)
  ctx.fillStyle = 'rgba(71, 85, 105, 0.25)'
  for (let i = 0; i < 300; i++) {
    ctx.fillRect(Math.random() * CW, Math.random() * CH, 1.5, 1.5)
  }
  ctx.fillStyle = '#475569'
  ctx.font = "bold 16px system-ui, sans-serif"
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(t('lotteryScratch'), CW / 2, CH / 2)
})

function scratchAt(e: PointerEvent) {
  const c = cv.value!
  const ctx = c.getContext('2d')!
  const r = c.getBoundingClientRect()
  const x = ((e.clientX - r.left) / r.width) * CW
  const y = ((e.clientY - r.top) / r.height) * CH
  ctx.globalCompositeOperation = 'destination-out'
  ctx.beginPath()
  ctx.arc(x, y, 18, 0, Math.PI * 2)
  ctx.fill()
  scratchTick()
  if (++strokes % 6 === 0) navigator.vibrate?.(4)
  if (strokes % 10 === 0) checkCleared(ctx)
}

function checkCleared(ctx: CanvasRenderingContext2D) {
  const data = ctx.getImageData(0, 0, CW, CH).data
  let clear = 0
  let total = 0
  // sample every 16th pixel's alpha channel
  for (let i = 3; i < data.length; i += 64) {
    total++
    if (data[i] === 0) clear++
  }
  if (clear / total > REVEAL_RATIO) finish()
}

const RAINBOW = ['#ff0084', '#fcff00', '#00fff0', '#7c00ff', '#ffffff']

function finish() {
  if (revealed.value) return
  revealed.value = true
  if (state.value) state.value.coins = props.finalCoins
  const cx = window.innerWidth / 2
  const cy = window.innerHeight / 2
  // celebration scales with the prize tier
  switch (LOTTERY_PRIZES.indexOf(props.prize)) {
    case 0: // jackpot: full fireworks
      play('win')
      confetti()
      burst(cx, cy, RAINBOW, 160)
      burst(cx, cy - 60, COIN_COLORS, 120)
      vibrate([50, 50, 50, 50, 200])
      break
    case 1:
      play('success_diamond')
      burst(cx, cy, COIN_COLORS, 120)
      vibrate([30, 30, 80])
      break
    case 2:
      play('success_gold')
      burst(cx, cy, COIN_COLORS, 60)
      vibrate([20, 30, 40])
      break
    case 3:
      play('success_silver')
      burst(cx, cy, COIN_COLORS, 30)
      vibrate(20)
      break
    default:
      play('switch')
  }
}

function onDown(e: PointerEvent) {
  scratching = true
  cv.value!.setPointerCapture(e.pointerId)
  scratchAt(e)
}

function onMove(e: PointerEvent) {
  if (scratching && !revealed.value) scratchAt(e)
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex flex-col items-center justify-center gap-4 bg-black/80 p-4 backdrop-blur-sm">
    <div
      class="card-in flex w-72 flex-col overflow-hidden rounded-xl border-2 border-dashed border-amber-300/70 bg-gradient-to-b from-rose-600 via-rose-700 to-rose-800 shadow-[0_0_40px_#f59e0b55]"
    >
      <div class="px-4 pt-3 text-center">
        <p class="text-2xl font-black tracking-widest text-amber-300">더또</p>
        <p class="text-[10px] uppercase tracking-[0.3em] text-rose-200">THE BUTTON {{ t('lotteryName') }}</p>
      </div>

      <div class="relative mx-4 my-3 h-28 overflow-hidden rounded-lg bg-slate-100">
        <div class="flex h-full flex-col items-center justify-center">
          <p v-if="prize > 0" class="text-xs font-bold text-rose-600">{{ tierLabel }}</p>
          <p class="text-3xl font-black" :class="prize > 0 ? 'text-amber-500' : 'text-slate-500'">
            {{ prize > 0 ? `💰${prize}` : `${t('lotteryLose')} 😢` }}
          </p>
        </div>
        <canvas
          ref="cv"
          :width="CW"
          :height="CH"
          class="absolute inset-0 h-full w-full touch-none transition-opacity duration-500"
          :class="revealed ? 'pointer-events-none opacity-0' : 'cursor-crosshair'"
          @pointerdown="onDown"
          @pointermove="onMove"
          @pointerup="scratching = false"
          @pointercancel="scratching = false"
        ></canvas>
      </div>

      <div class="mx-4 mb-3 flex h-5 items-end justify-center gap-[2px] opacity-70">
        <span
          v-for="i in 28"
          :key="i"
          class="bg-amber-100"
          :style="{ width: `${((i * 7) % 3) + 1}px`, height: `${((i * 5) % 8) + 10}px` }"
        ></span>
      </div>
    </div>

    <button
      v-if="revealed"
      class="rounded-full border border-slate-600 px-6 py-1.5 text-sm text-slate-300 hover:bg-slate-800"
      @click="$emit('close')"
    >
      OK
    </button>
    <p v-else class="text-xs text-slate-400">{{ t('lotteryScratch') }}</p>
  </div>
</template>
