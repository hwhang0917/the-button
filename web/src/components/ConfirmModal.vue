<script setup lang="ts">
defineProps<{
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
}>()
defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    @click.self="$emit('cancel')"
  >
    <div class="flex w-80 flex-col gap-4 rounded-xl border border-slate-700 bg-slate-900 p-6">
      <h2 class="text-center text-lg font-bold text-slate-100">{{ title }}</h2>
      <p class="text-center text-sm leading-relaxed text-slate-300">{{ message }}</p>
      <div class="flex gap-2">
        <button
          class="flex-1 rounded-lg border border-slate-600 py-2 text-sm text-slate-400 hover:bg-slate-800"
          @click="$emit('cancel')"
        >
          {{ cancelLabel }}
        </button>
        <button
          data-space
          class="flex-1 rounded-lg bg-rose-500 py-2 text-sm font-bold text-white hover:bg-rose-400"
          @click="$emit('confirm')"
        >
          {{ confirmLabel }}
        </button>
      </div>
    </div>
  </div>
</template>
