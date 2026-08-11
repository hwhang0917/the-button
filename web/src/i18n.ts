import { ref } from 'vue'

const dict = {
  ko: {
    subtitle: '강화 클리커 — 어디까지 올라갈 수 있을까?',
    chance: '성공 확률',
    press: '강화',
    riskIt: '리스크 모드',
    riskDesc: '확률 ½ · 성공 시 ★+2',
    clicksLeft: '남은 시도',
    quotaExhausted: '시도를 다 썼어요. 다음 정각에 다시 오세요!',
    success: '강화 성공!',
    tierUp: '티어 승급!',
    fail: '강화 실패… 별이 모두 사라졌습니다',
    win: '★15 달성! 전설이 되었습니다',
    leaderboard: '랭킹',
    collection: '컬렉션',
    cardDrop: '카드 획득!',
    nicknameTitle: '랭킹에 이름을 올리세요',
    nicknamePlaceholder: '닉네임 (최대 16자)',
    save: '저장',
    later: '나중에',
    best: '최고 기록',
    empty: '아직 아무도 없어요',
    changeName: '이름 변경',
    deleteData: '기록 삭제',
    deleteConfirm:
      '정말 삭제할까요? 랭킹 기록과 카드가 모두 사라집니다. 남은 시도 횟수는 IP 기준이라 초기화되지 않아요.',
    tier: { unrank: '언랭', bronze: '브론즈', silver: '실버', gold: '골드', platinum: '플래티넘', diamond: '다이아몬드' },
    rarity: { common: '커먼', rare: '레어', holo: '홀로', prismatic: '프리즘' },
  },
  en: {
    subtitle: 'enchant clicker — how far can your streak go?',
    chance: 'Success rate',
    press: 'ENCHANT',
    riskIt: 'RISK IT',
    riskDesc: '½ odds · ★+2 on success',
    clicksLeft: 'clicks left',
    quotaExhausted: 'Out of clicks — back on the hour!',
    success: 'Success!',
    tierUp: 'TIER UP!',
    fail: 'Failed… all stars lost',
    win: '★15 — you are a legend',
    leaderboard: 'Ranking',
    collection: 'Collection',
    cardDrop: 'Card drop!',
    nicknameTitle: 'Put your name on the board',
    nicknamePlaceholder: 'Nickname (max 16 chars)',
    save: 'Save',
    later: 'Later',
    best: 'Best',
    empty: 'Nobody yet',
    changeName: 'Change name',
    deleteData: 'Delete data',
    deleteConfirm:
      'Really delete? Your rank and cards will be gone. Remaining clicks are per-IP and will NOT reset.',
    tier: { unrank: 'Unrank', bronze: 'Bronze', silver: 'Silver', gold: 'Gold', platinum: 'Platinum', diamond: 'Diamond' },
    rarity: { common: 'Common', rare: 'Rare', holo: 'Holo', prismatic: 'Prismatic' },
  },
} as const

export type Lang = keyof typeof dict

export const lang = ref<Lang>(
  (localStorage.getItem('lang') as Lang | null) ??
    (navigator.language.startsWith('en') ? 'en' : 'ko'),
)

export function toggleLang() {
  lang.value = lang.value === 'ko' ? 'en' : 'ko'
  localStorage.setItem('lang', lang.value)
}

type Messages = (typeof dict)['ko']

export function t<K extends keyof Messages>(key: K): Messages[K] {
  return dict[lang.value][key] as Messages[K]
}
