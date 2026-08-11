<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { newLinkCode, claimLinkCode } from '../useGame'
import { t } from '../i18n'

defineEmits<{ close: [] }>()

const myCode = ref('')
const code = ref('')
const claiming = ref(false)
const failed = ref(false)

onMounted(async () => {
  myCode.value = (await newLinkCode()) ?? '—'
})

async function claim() {
  const c = code.value.trim()
  if (!c || claiming.value) return
  claiming.value = true
  failed.value = false
  if (await claimLinkCode(c)) {
    location.reload()
    return
  }
  failed.value = true
  claiming.value = false
}
</script>

<template>
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 backdrop-blur-sm" @click.self="$emit('close')">
    <form
      class="flex w-80 flex-col gap-4 rounded-xl border border-slate-700 bg-slate-900 p-6"
      @submit.prevent="claim"
    >
      <h2 class="text-center text-lg font-bold text-slate-100">🔗 {{ t('linkTitle') }}</h2>

      <p class="text-center font-mono text-2xl font-bold tracking-widest text-yellow-300">{{ myCode }}</p>
      <p class="text-center text-xs text-slate-500">{{ t('linkCodeHint') }}</p>

      <hr class="border-slate-700" />

      <input
        v-model="code"
        :placeholder="t('linkPlaceholder')"
        maxlength="8"
        class="rounded-lg border border-slate-600 bg-slate-800 px-3 py-2 text-center font-mono uppercase tracking-widest text-slate-100 outline-none focus:border-yellow-400"
      />
      <p class="text-center text-xs" :class="failed ? 'text-rose-400' : 'text-slate-500'">
        {{ failed ? t('linkBadCode') : t('linkReplaceWarn') }}
      </p>
      <div class="flex gap-2">
        <button
          type="button"
          class="flex-1 rounded-lg border border-slate-600 py-2 text-sm text-slate-400 hover:bg-slate-800"
          @click="$emit('close')"
        >
          {{ t('later') }}
        </button>
        <button
          type="submit"
          :disabled="!code.trim() || claiming"
          class="flex-1 rounded-lg bg-yellow-400 py-2 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-40"
        >
          {{ t('linkSubmit') }}
        </button>
      </div>
    </form>
  </div>
</template>
