<script setup lang="ts">
defineProps<{ stars: number }>()
const MAX = 15
</script>

<template>
  <div class="flex flex-wrap justify-center gap-1 max-w-xs">
    <img
      v-for="i in MAX"
      :key="`${i}-${i <= stars}`"
      src="/star.png"
      alt="★"
      class="h-5 w-5 transition-all"
      :class="i <= stars ? 'star-pop drop-shadow-[0_0_6px_#facc15]' : 'opacity-15 grayscale'"
    />
  </div>
</template>
