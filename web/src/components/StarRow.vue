<script setup lang="ts">
import { computed } from 'vue'
import { PRESTIGE_SKIN_CAP } from '../useGame'

const props = withDefaults(defineProps<{ stars: number; prestige?: number }>(), { prestige: 0 })
const MAX = 15
// visuals cap at prismatic even though prestige keeps counting
const skin = computed(() => Math.min(props.prestige, PRESTIGE_SKIN_CAP))
</script>

<template>
  <div class="flex flex-wrap justify-center gap-1 max-w-xs">
    <img
      v-for="i in MAX"
      :key="`${i}-${i <= stars}`"
      src="/star.png"
      alt="★"
      class="h-5 w-5 transition-all"
      :class="
        i <= stars
          ? ['star-pop', skin > 0 ? `star-r${skin}` : 'drop-shadow-[0_0_6px_#facc15]']
          : 'opacity-15 grayscale'
      "
    />
  </div>
</template>
