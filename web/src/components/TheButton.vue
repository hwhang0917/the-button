<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Application, Container, Graphics, Sprite, Text, Texture } from 'pixi.js'
import { TIER_COLORS, type Tier } from '../tiers'
import { t, lang } from '../i18n'
import { play, vibrate } from '../audio'

const props = withDefaults(
  defineProps<{
    tier: Tier
    chance: number
    risky: boolean
    disabled: boolean
    prestige?: number
    shield?: boolean
    talisman?: boolean
    bonus?: number
  }>(),
  { prestige: 0, shield: false, talisman: false, bonus: 0 },
)
const emit = defineEmits<{ press: [center: { x: number; y: number }] }>()

const host = ref<HTMLDivElement | null>(null)

const SIZE = 280
const R = 88

let app: Application | null = null
let btn: Container
let fx: Container
let halo: Graphics
let auraShield: Graphics
let auraTalisman: Graphics
let base: Graphics
let shadeDark: Graphics
let shadeHi: Graphics
let rim: Graphics
let label: Text
let pct: Text
let emberTex: Texture

// spring physics: scale and positional offset both spring back to rest
let scale = 1
let vScale = 0
let targetScale = 1
let ox = 0
let oy = 0
let vx = 0
let vy = 0
let phase = 0
let pressed = false
let emberAcc = 0
// purely-aesthetic press charge: holding compresses and trembles the button,
// releasing pops it harder the longer it was held. never touches the outcome.
let holdFrames = 0
const CHARGE_FRAMES = 90 // ~1.5s to full squash

interface Ember {
  sp: Sprite
  vx: number
  vy: number
  life: number
  max: number
}
const embers: Ember[] = []
const freeSprites: Sprite[] = []

// amber → deep red; hotter picks shift right
const EMBER_COLORS = [0xfcd34d, 0xfb923c, 0xf87171, 0xef4444]
// rare-prestige embers burn cold blue instead
const EMBER_COLORS_RARE = [0xbae6fd, 0x7dd3fc, 0x38bdf8, 0x0ea5e9]

let hue = 0
let sparkles: Sprite[] = []

// hsl(h, 85%, 60%) → rgb int, for the prestige rainbow tints
function hslTint(h: number): number {
  const s = 0.85
  const l = 0.6
  const c = (1 - Math.abs(2 * l - 1)) * s
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = l - c / 2
  const [r, g, b] =
    h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x]
  return (Math.round((r + m) * 255) << 16) | (Math.round((g + m) * 255) << 8) | Math.round((b + m) * 255)
}

// 0..1: embers start under ~60% odds and rage as the odds shrink
function heat(): number {
  return Math.max(0, Math.min(1, (60 - props.chance) / 55))
}

// aura rings signalling attached consumables: 🛡 sky blue, 🃏 amber
const AURA_SHIELD_TINT = 0x38bdf8
const AURA_TALISMAN_TINT = 0xfbbf24

// a dashed ring drawn white so .tint can color it; rotated/pulsed in the ticker
function drawAuraRing(g: Graphics, radius: number) {
  g.clear()
  const SEGS = 5
  for (let i = 0; i < SEGS; i++) {
    const a0 = (i * Math.PI * 2) / SEGS
    const a1 = a0 + ((Math.PI * 2) / SEGS) * 0.55
    g.moveTo(Math.cos(a0) * radius, Math.sin(a0) * radius)
    g.arc(0, 0, radius, a0, a1)
  }
  g.stroke({ width: 4, color: 0xffffff, cap: 'round' })
}

function drawButton() {
  const color = TIER_COLORS[props.tier]
  // holo/prismatic prestige draws the edge white and hue-cycles it via .tint;
  // rare prestige is a fixed cool blue; risky red wins below that
  const rainbow = props.prestige >= 2
  const edge = rainbow ? '#ffffff' : props.risky ? '#f43f5e' : props.prestige === 1 ? '#38bdf8' : color
  if (!rainbow) {
    rim.tint = 0xffffff
    halo.tint = 0xffffff
  }
  halo.clear()
  const haloAlpha = props.prestige >= 3 ? 0.075 : 0.045
  for (let i = 3; i >= 1; i--) {
    halo.circle(0, 0, R + i * 13).fill({ color: edge, alpha: haloAlpha * (4 - i) })
  }
  base.clear()
  base.circle(0, 0, R).fill(color)
  shadeDark.clear()
  shadeDark.circle(R * 0.3, R * 0.4, R * 1.15).fill({ color: '#0b1120', alpha: 0.5 })
  shadeHi.clear()
  shadeHi.circle(-R * 0.3, -R * 0.35, R * 0.8).fill({ color: '#ffffff', alpha: 0.14 })
  rim.clear()
  rim.circle(0, 0, R).stroke({ width: 4, color: edge })
}

function syncTexts() {
  label.text = t('press')
  // the talisman bonus rides the roll only, so it shows as its own +N% tag
  pct.text = props.bonus > 0 ? `${props.chance}% +${props.bonus}%` : `${props.chance}%`
  pct.style.fill = props.bonus > 0 ? '#fcd34d' : props.risky ? '#fda4af' : '#e2e8f0'
}

function applyDisabled() {
  btn.eventMode = props.disabled ? 'none' : 'static'
  btn.alpha = props.disabled ? 0.35 : 1
}

function spawnEmber() {
  const sp = freeSprites.pop() ?? new Sprite(emberTex)
  sp.anchor.set(0.5)
  sp.blendMode = 'add'
  const a = Math.random() * Math.PI * 2
  const rr = R * (0.9 + Math.random() * 0.25) * scale
  sp.position.set(btn.position.x + Math.cos(a) * rr, btn.position.y + Math.sin(a) * rr)
  const h = heat()
  if (props.prestige >= 2) {
    sp.tint = hslTint(Math.random() * 360)
  } else {
    const palette = props.prestige === 1 && !props.risky ? EMBER_COLORS_RARE : EMBER_COLORS
    const hot = Math.random() * 2 + (props.risky ? 2 : h * 2)
    sp.tint = palette[Math.min(palette.length - 1, Math.floor(hot))]
  }
  sp.alpha = 1
  const max = 30 + Math.random() * 30
  embers.push({
    sp,
    vx: (Math.random() - 0.5) * 0.6,
    vy: -(0.6 + Math.random() * 1.4) * (0.6 + h),
    life: max,
    max,
  })
  fx.addChild(sp)
}

onMounted(async () => {
  const a = new Application()
  await a.init({
    width: SIZE,
    height: SIZE,
    backgroundAlpha: 0,
    antialias: true,
    resolution: Math.min(window.devicePixelRatio || 1, 2),
    autoDensity: true,
  })
  app = a
  host.value!.appendChild(a.canvas)
  // native listener: some Android browsers ignore vibrate() from Pixi's
  // synthesized pointer events, but credit a real touchstart as the gesture
  a.canvas.addEventListener('touchstart', () => vibrate(22), { passive: true })

  emberTex = a.renderer.generateTexture(new Graphics().circle(0, 0, 4).fill('#ffffff'))

  fx = new Container()
  btn = new Container()
  halo = new Graphics()
  auraShield = new Graphics()
  auraShield.tint = AURA_SHIELD_TINT
  auraShield.alpha = 0
  drawAuraRing(auraShield, R + 20)
  auraTalisman = new Graphics()
  auraTalisman.tint = AURA_TALISMAN_TINT
  auraTalisman.alpha = 0
  drawAuraRing(auraTalisman, R + 30)
  base = new Graphics()
  shadeDark = new Graphics()
  shadeHi = new Graphics()
  rim = new Graphics()

  // layered circles under a circular mask fake the radial gradient
  const shade = new Container()
  const mask = new Graphics().circle(0, 0, R).fill('#ffffff')
  shade.addChild(base, shadeDark, shadeHi, mask)
  shade.mask = mask

  label = new Text({
    text: '',
    style: { fontFamily: 'system-ui, sans-serif', fontSize: 26, fontWeight: '900', fill: '#ffffff', letterSpacing: 4 },
  })
  label.anchor.set(0.5)
  label.position.set(0, -8)
  pct = new Text({
    text: '',
    // padding: glyphs (%) can render past the measured width and get clipped
    style: { fontFamily: 'ui-monospace, monospace', fontSize: 16, fontWeight: '700', fill: '#e2e8f0', padding: 4 },
  })
  pct.anchor.set(0.5)
  pct.position.set(0, 24)

  btn.addChild(halo, auraShield, auraTalisman, shade, rim, label, pct)
  btn.position.set(SIZE / 2, SIZE / 2)
  btn.cursor = 'pointer'
  a.stage.addChild(fx, btn)

  drawButton()
  syncTexts()
  applyDisabled()

  btn.on('pointerover', () => {
    play('mouseover')
    targetScale = 1.06
  })
  btn.on('pointerout', () => {
    targetScale = 1
    pressed = false
  })
  btn.on('pointerdown', () => {
    pressed = true
    holdFrames = 0
    targetScale = 0.82
    play('click')
    vibrate(22) // punchy press-down thunk (overrides the click sound's buzz)
    vy += 1.5
    for (let i = 0; i < 6; i++) spawnEmber() // impact sparks
  })
  const release = () => {
    const charge = Math.min(holdFrames / CHARGE_FRAMES, 1)
    targetScale = 1
    pressed = false
    holdFrames = 0
    vScale += 0.06 + 0.2 * charge // bigger pop the longer the hold
    if (charge > 0.15) {
      vibrate(Math.round(8 + 30 * charge))
    } else {
      vibrate(6) // light tick even on a quick tap
    }
    // release always sparks; a charged hold erupts
    for (let i = Math.max(4, Math.round(charge * 20)); i > 0; i--) spawnEmber()
  }
  btn.on('pointerupoutside', release)
  btn.on('pointerup', () => {
    if (!pressed) return
    release()
    const r = host.value!.getBoundingClientRect()
    emit('press', { x: r.left + r.width / 2, y: r.top + r.height / 2 })
  })

  a.ticker.add((tk) => {
    const dt = Math.min(tk.deltaTime, 3)
    phase += dt * 0.035
    const damp = Math.pow(0.82, dt)

    vScale += (targetScale - scale) * 0.16 * dt
    vScale *= damp
    scale += vScale * dt
    vx += -ox * 0.1 * dt
    vx *= damp
    ox += vx * dt
    vy += -oy * 0.1 * dt
    vy *= damp
    oy += vy * dt

    // holding: squash deeper over time and tremble like a compressed spring
    let charge = 0
    if (pressed) {
      holdFrames += dt
      charge = Math.min(holdFrames / CHARGE_FRAMES, 1)
      targetScale = 0.82 - 0.12 * charge
    }
    const tremX = (Math.random() - 0.5) * 3 * charge
    const tremY = (Math.random() - 0.5) * 3 * charge

    const breath = 1 + Math.sin(phase * 1.7) * 0.012
    btn.scale.set(scale * breath)
    btn.position.set(SIZE / 2 + ox + tremX, SIZE / 2 + oy + tremY)
    btn.rotation = Math.sin(phase * 0.9) * 0.02

    // consumable auras: counter-rotating pulsing rings while attached
    auraShield.rotation = phase * 0.6
    auraShield.alpha = props.shield ? 0.4 + Math.sin(phase * 2.2) * 0.2 : 0
    auraTalisman.rotation = -phase * 0.8
    auraTalisman.alpha = props.talisman ? 0.4 + Math.sin(phase * 2.6 + 1) * 0.2 : 0

    // prestige flair: hue-cycled rim/halo for holo+, orbiting sparkles for prismatic
    if (props.prestige >= 2) {
      hue = (hue + dt * (props.prestige >= 3 ? 2.4 : 0.8)) % 360
      const tint = hslTint(hue)
      rim.tint = tint
      halo.tint = tint
    }
    if (props.prestige >= 3) {
      if (!sparkles.length) {
        for (let i = 0; i < 4; i++) {
          const sp = new Sprite(emberTex)
          sp.anchor.set(0.5)
          sp.blendMode = 'add'
          fx.addChild(sp)
          sparkles.push(sp)
        }
      }
      sparkles.forEach((sp, i) => {
        const ang = phase * 0.8 + (i * Math.PI * 2) / sparkles.length
        sp.visible = true
        sp.position.set(
          btn.position.x + Math.cos(ang) * (R + 16) * scale,
          btn.position.y + Math.sin(ang) * (R + 16) * scale,
        )
        sp.tint = hslTint((hue + i * 90) % 360)
        sp.alpha = 0.7 + Math.sin(phase * 3 + i) * 0.3
        sp.scale.set(0.5 + 0.2 * Math.sin(phase * 2 + i))
      })
    } else {
      sparkles.forEach((sp) => (sp.visible = false))
    }

    // ember emission scales with how bad the odds are, plus a charging sizzle
    if (!props.disabled) {
      emberAcc += dt * (heat() * (props.risky ? 0.9 : 0.55) + charge * 0.4)
      while (emberAcc >= 1) {
        emberAcc -= 1
        spawnEmber()
      }
    }
    for (let i = embers.length - 1; i >= 0; i--) {
      const e = embers[i]
      e.life -= dt
      e.sp.x += (e.vx + Math.sin((e.life + i) * 0.15) * 0.35) * dt
      e.sp.y += e.vy * dt
      e.vy -= 0.01 * dt // embers accelerate upward as they cool
      const lr = Math.max(e.life / e.max, 0)
      e.sp.alpha = lr
      e.sp.scale.set(0.35 + lr * 0.65)
      if (e.life <= 0) {
        fx.removeChild(e.sp)
        freeSprites.push(e.sp)
        embers.splice(i, 1)
      }
    }
  })
})

watch(() => [props.tier, props.risky, props.prestige], () => app && drawButton())
watch(() => [props.chance, props.risky, props.bonus, lang.value], () => app && syncTexts())
watch(() => props.disabled, () => app && applyDisabled())

onBeforeUnmount(() => app?.destroy(true, { children: true }))
</script>

<template>
  <div ref="host" class="h-[280px] w-[280px] select-none" />
</template>
