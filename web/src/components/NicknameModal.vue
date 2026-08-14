<script setup lang="ts">
import { computed, ref } from 'vue'
import { saveNickname, loadLeaderboard, state } from '../useGame'
import { cfg } from '../config'
import { t } from '../i18n'

const emit = defineEmits<{ close: []; link: [] }>()
// no nickname yet = the signup gate: not dismissable, anonymous play is off
const gate = computed(() => !state.value?.nickname)
const name = ref('')
const saving = ref(false)
const taken = ref(false)
// same pattern the server builds from config; lengths come from /api/config
const placeholder = computed(() =>
  t('nicknamePlaceholder')
    .replace('{min}', String(cfg().nickname.minLen))
    .replace('{max}', String(cfg().nickname.maxLen)),
)
const valid = computed(() =>
  new RegExp(`^[A-Za-z0-9_가-힣]{${cfg().nickname.minLen},${cfg().nickname.maxLen}}$`).test(
    name.value.trim(),
  ),
)

async function submit() {
  const n = name.value.trim()
  if (!valid.value || saving.value) return
  saving.value = true
  taken.value = false
  const result = await saveNickname(n)
  if (result === 'ok') {
    await loadLeaderboard()
    emit('close')
  }
  taken.value = result === 'taken'
  saving.value = false
}
</script>

<template>
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/70 backdrop-blur-sm" @click.self="!gate && $emit('close')">
    <form
      class="flex w-80 flex-col gap-4 rounded-xl border border-slate-700 bg-slate-900 p-6"
      @submit.prevent="submit"
    >
      <h2 class="text-center text-lg font-bold text-slate-100">
        {{ gate ? `🚀 ${t('signupTitle')}` : `🏆 ${t('nicknameTitle')}` }}
      </h2>
      <input
        v-model="name"
        :placeholder="placeholder"
        maxlength="16"
        autofocus
        class="rounded-lg border border-slate-600 bg-slate-800 px-3 py-2 text-slate-100 outline-none focus:border-yellow-400"
      />
      <p v-if="taken" class="text-center text-xs text-rose-400">{{ t('nameTaken') }}</p>
      <div class="flex gap-2">
        <button
          v-if="!gate"
          type="button"
          class="flex-1 rounded-lg border border-slate-600 py-2 text-sm text-slate-400 hover:bg-slate-800"
          @click="$emit('close')"
        >
          {{ t('later') }}
        </button>
        <button
          type="submit"
          :disabled="!valid || saving"
          class="flex-1 rounded-lg bg-yellow-400 py-2 text-sm font-bold text-black hover:bg-yellow-300 disabled:opacity-40"
        >
          {{ gate ? t('signupSubmit') : t('save') }}
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
