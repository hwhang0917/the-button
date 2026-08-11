import { ref } from 'vue'

const dict = {
  ko: {
    subtitle: '강화 클리커 — 어디까지 올라갈 수 있을까?',
    loading: '로딩 중…',
    chance: '성공 확률',
    press: '강화',
    riskIt: '리스크 모드',
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
    nicknamePlaceholder: '닉네임 (영문·숫자·_ 3~16자)',
    setName: '이름 설정',
    nameTaken: '이미 사용 중인 이름이에요',
    save: '저장',
    later: '나중에',
    best: '최고 기록',
    empty: '아직 아무도 없어요',
    changeName: '이름 변경',
    deleteData: '기록 삭제',
    deleteConfirm: '정말 삭제할까요? 랭킹 기록과 카드가 모두 사라지며 되돌릴 수 없어요.',
    linkDevice: '기기 연결',
    linkTitle: '다른 기기와 연결',
    linkCodeHint: '다른 기기에서 이 코드를 입력하세요 · 10분간 유효',
    linkPlaceholder: '코드 입력',
    linkSubmit: '연결',
    linkBadCode: '코드가 올바르지 않거나 만료되었어요',
    linkReplaceWarn: '연결하면 이 기기의 현재 진행 상황을 대체해요',
    linkHave: '연결 코드가 있나요?',
    shop: '스킬 상점',
    sellStreak: '스트릭 판매',
    sellDesc: '현재 별을 코인으로 바꾸고 처음부터 다시 시작해요',
    maxLevel: 'MAX',
    shieldName: '보호 주문서',
    shieldDesc: '실패해도 별 유지 (1회용)',
    charmName: '행운 부적',
    charmDesc: '성공 확률 +2% / 레벨',
    headstartName: '출발 부스트',
    headstartDesc: '리셋 시 ★레벨부터 시작',
    shieldSaved: '🛡️ 보호 주문서가 별을 지켰어요!',
    privacy: '개인정보 처리방침',
    privacyBody:
      '이 게임은 개인정보를 수집하지 않습니다.\n\n' +
      '• 계정은 무작위 익명 토큰 쿠키(bt_token)가 전부입니다 — 이메일·비밀번호·실명 없이 동작해요.\n' +
      '• 서버에는 진행 상황(별, 카드, 직접 정한 닉네임)만 이 토큰에 묶여 저장됩니다.\n' +
      '• 게임 서버는 IP 주소를 저장하지 않습니다.\n' +
      '• 유일한 개인정보 접점: 앞단의 리버스 프록시가 과도한 요청을 막기 위해 IP 기준 속도 제한을 수행하며, IP는 그곳에서만 잠시 처리됩니다.\n' +
      '• 플레이어 메뉴의 "기록 삭제"로 언제든 모든 데이터를 지울 수 있습니다.',
    tier: { unrank: '언랭', bronze: '브론즈', silver: '실버', gold: '골드', platinum: '플래티넘', diamond: '다이아몬드' },
    rarity: { common: '커먼', rare: '레어', holo: '홀로', prismatic: '프리즘' },
  },
  en: {
    subtitle: 'enchant clicker — how far can your streak go?',
    loading: 'Loading…',
    chance: 'Success rate',
    press: 'ENCHANT',
    riskIt: 'RISK IT',
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
    nicknamePlaceholder: 'Nickname (a-z, 0-9, _ · 3-16 chars)',
    setName: 'Set name',
    nameTaken: 'That name is already taken',
    save: 'Save',
    later: 'Later',
    best: 'Best',
    empty: 'Nobody yet',
    changeName: 'Change name',
    deleteData: 'Delete data',
    deleteConfirm: 'Really delete? Your rank and cards will be gone for good.',
    linkDevice: 'Link device',
    linkTitle: 'Link another device',
    linkCodeHint: 'Enter this code on your other device · valid 10 minutes',
    linkPlaceholder: 'Enter code',
    linkSubmit: 'Link',
    linkBadCode: 'Code is wrong or expired',
    linkReplaceWarn: 'Linking replaces this device’s current progress',
    linkHave: 'Have a link code?',
    shop: 'Skill shop',
    sellStreak: 'Sell streak',
    sellDesc: 'Trade your current stars for coins and start over',
    maxLevel: 'MAX',
    shieldName: 'Protection scroll',
    shieldDesc: 'Keep your stars on a fail (single use)',
    charmName: 'Lucky charm',
    charmDesc: '+2% success chance per level',
    headstartName: 'Head start',
    headstartDesc: 'Resets land at ★level instead of ★0',
    shieldSaved: '🛡️ Your protection scroll saved the stars!',
    privacy: 'Privacy policy',
    privacyBody:
      'This game collects no personal data.\n\n' +
      '• Your account is nothing but a random anonymous token cookie (bt_token) — no email, password, or real name.\n' +
      '• The server stores only game progress (stars, cards, a nickname you choose) keyed to that token.\n' +
      '• The game server never stores IP addresses.\n' +
      '• The only PII touchpoint: the reverse proxy in front rate-limits requests per IP, so IPs are briefly processed there and nowhere else.\n' +
      '• "Delete data" in the player menu erases everything, any time.',
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
