<script setup lang="ts">
import { computed, ref } from 'vue'
import { saveNickname, loadLeaderboard } from '../useGame'
import { t } from '../i18n'

const emit = defineEmits<{ close: []; link: [] }>()
const name = ref('')
const saving = ref(false)
// mirrors nicknameRe in main.go
const valid = computed(() => /^[A-Za-z0-9_]{3,16}$/.test(name.value.trim()))

async function submit() {
  const n = name.value.trim()
  if (!valid.value || saving.value) return
  saving.value = true
  if (await saveNickname(n)) {
    await loadLeaderboard()
    emit('close')
  }
  saving.value = false
}
</script>

<template>
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 backdrop-blur-sm" @click.self="$emit('close')">
    <form
      class="flex w-80 flex-col gap-4 rounded-xl border border-slate-700 bg-slate-900 p-6"
      @submit.prevent="submit"
    >
      <h2 class="text-center text-lg font-bold text-slate-100">🏆 {{ t('nicknameTitle') }}</h2>
      <input
        v-model="name"
        :placeholder="t('nicknamePlaceholder')"
        maxlength="16"
        autofocus
        class="rounded-lg border border-slate-600 bg-slate-800 px-3 py-2 text-slate-100 outline-none focus:border-yellow-400"
      />
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
          :disabled="!valid || saving"
          class="flex-1 rounded-lg bg-yellow-400 py-2 text-sm font-bold text-slate-900 hover:bg-yellow-300 disabled:opacity-40"
        >
          {{ t('save') }}
        </button>
      </div>
      <button
        type="button"
        class="text-center text-xs text-slate-500 underline hover:text-slate-300"
        @click="$emit('link')"
      >
        🔗 {{ t('linkHave') }}
      </button>
    </form>
  </div>
</template>
