<script setup lang="ts">
import { ref } from 'vue'
import { TIER_COLORS, type Tier } from '../tiers'
import { t } from '../i18n'
import { play } from '../audio'

const props = defineProps<{
  tier: Tier
  chance: number
  risky: boolean
  disabled: boolean
}>()
const emit = defineEmits<{ press: [center: { x: number; y: number }] }>()

const el = ref<HTMLButtonElement | null>(null)
const pressing = ref(false)

function onClick() {
  if (props.disabled) return
  play('click')
  pressing.value = true
  setTimeout(() => (pressing.value = false), 120)
  const r = el.value!.getBoundingClientRect()
  emit('press', { x: r.left + r.width / 2, y: r.top + r.height / 2 })
}
</script>

<template>
  <button
    ref="el"
    class="relative h-44 w-44 rounded-full font-black text-2xl tracking-widest text-white transition-transform select-none sm:h-52 sm:w-52"
    :class="[
      pressing ? 'scale-90' : 'hover:scale-105 active:scale-95',
      disabled ? 'cursor-not-allowed opacity-40 grayscale' : 'cursor-pointer',
    ]"
    :style="{
      background: `radial-gradient(circle at 35% 30%, ${TIER_COLORS[tier]}cc, #111827 80%)`,
      boxShadow: risky
        ? `0 0 60px #f43f5eaa, inset 0 0 30px #0008`
        : `0 0 40px ${TIER_COLORS[tier]}77, inset 0 0 30px #0008`,
      border: `3px solid ${risky ? '#f43f5e' : TIER_COLORS[tier]}`,
    }"
    :disabled="disabled"
    @click="onClick"
    @mouseenter="!disabled && play('mouseover')"
  >
    {{ t('press') }}
    <span
      class="absolute -bottom-2 left-1/2 -translate-x-1/2 rounded-full px-3 py-0.5 text-xs font-bold"
      :class="risky ? 'bg-rose-600' : 'bg-slate-800'"
    >
      {{ chance }}%
    </span>
  </button>
</template>
