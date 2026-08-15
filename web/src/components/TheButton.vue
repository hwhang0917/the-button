<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Application, Container, Graphics, Sprite, Text, Texture } from 'pixi.js'
import { TIER_COLORS, type Tier } from '../tiers'
import { t, lang } from '../i18n'
import { play, vibrate } from '../audio'
import { reducedMotion } from '../particles'

const props = withDefaults(
  defineProps<{
    tier: Tier
    chance: number
    /** 0..maxRisk — drives the flames */
    risk: number
    /** how deep the streak is, 0..1 — drives the steam */
    pressure?: number
    /** the highest risk level on offer, so flames scale to the ladder */
    maxRisk?: number
    disabled: boolean
    /** max stars reached: still unclickable, but celebrating, not dimmed */
    win?: boolean
    prestige?: number
    talisman?: boolean
    bonus?: number
  }>(),
  { win: false, prestige: 0, talisman: false, bonus: 0, pressure: 0, maxRisk: 3 },
)

const risky = () => props.risk > 0
const emit = defineEmits<{ press: [center: { x: number; y: number }] }>()

const host = ref<HTMLDivElement | null>(null)

const SIZE = 280
const R = 88

let app: Application | null = null
let btn: Container
let fx: Container
let halo: Graphics
let auraTalisman: Graphics
let base: Graphics
let shadeDark: Graphics
let shadeHi: Graphics
let rim: Graphics
let label: Text
let pct: Text
let emberTex: Texture
let steamTex: Texture
let flameOuter: Graphics
let flameCore: Graphics

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

// steam: slow vapour puffs that rise off the top of an overheating streak.
// Deliberately NOT additive — light would read as more fire; vapour should
// wash the background out instead.
interface Puff {
  sp: Sprite
  vx: number
  vy: number
  life: number
  max: number
}
const puffs: Puff[] = []
const freePuffs: Sprite[] = []
let steamAcc = 0

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

// Flames answer for the risk you opted into and steam for how far the streak
// has climbed, so the two effects stay readable as separate warnings instead of
// one indistinct blaze. Both ease in so the first rung is a hint, not a bonfire.
function flameLevel(): number {
  return props.maxRisk > 0 ? Math.min(1, props.risk / props.maxRisk) : 0
}
function steamLevel(): number {
  // nothing until the streak is genuinely deep, then ramps to full at the cap
  return Math.max(0, Math.min(1, (props.pressure - 0.35) / 0.65))
}

const FLAME_TONGUES = 7

// Tongues anchored around the lower rim, licking outward and biased upward.
// Redrawn every frame: each tongue carries its own phase offset so the sheet
// flickers rather than pulsing as one body.
function drawFlames(g: Graphics, level: number, color: string, reach: number, width: number, alpha: number) {
  g.clear()
  if (level <= 0) return
  const spread = Math.PI * 0.72
  const start = Math.PI * 0.14
  for (let i = 0; i < FLAME_TONGUES; i++) {
    const ang = start + (i / (FLAME_TONGUES - 1)) * spread
    const dx = Math.cos(ang)
    const dy = Math.sin(ang)
    // two out-of-step sines per tongue keep the flicker from looking periodic
    const flicker = 0.55 + 0.45 * Math.sin(phase * 7 + i * 1.9) * Math.sin(phase * 3.1 + i)
    const len = R * reach * level * flicker
    const w = R * width * (0.6 + 0.4 * level)
    // perpendicular to the radius, for the tongue's base width
    const px = -dy * w
    const py = dx * w
    const rise = len * 0.9 // flames lick upward wherever they are rooted
    const midx = dx * (R + len * 0.55)
    const midy = dy * (R + len * 0.55) - rise * 0.45
    g.moveTo(dx * R + px, dy * R + py)
    g.quadraticCurveTo(midx + px * 0.45, midy + py * 0.45, dx * (R + len) * 0.92, dy * (R + len) - rise)
    g.quadraticCurveTo(midx - px * 0.45, midy - py * 0.45, dx * R - px, dy * R - py)
    g.closePath()
  }
  g.fill({ color, alpha: alpha * (0.45 + 0.55 * level) })
}

function spawnPuff() {
  if (reducedMotion.matches) return
  const sp = freePuffs.pop() ?? new Sprite(steamTex)
  sp.anchor.set(0.5)
  // vapour off the upper rim, wandering a little to either side
  const ang = -Math.PI / 2 + (Math.random() - 0.5) * Math.PI * 0.8
  const rr = R * (0.75 + Math.random() * 0.3) * scale
  sp.position.set(btn.position.x + Math.cos(ang) * rr, btn.position.y + Math.sin(ang) * rr)
  sp.tint = 0xe2e8f0
  sp.alpha = 0
  sp.scale.set(0.5)
  const max = 70 + Math.random() * 50
  puffs.push({
    sp,
    vx: (Math.random() - 0.5) * 0.5,
    vy: -(0.35 + Math.random() * 0.45),
    life: max,
    max,
  })
  fx.addChild(sp)
}

// amber aura ring signalling an armed 🃏 card
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
  const edge = rainbow ? '#ffffff' : risky() ? '#f43f5e' : props.prestige === 1 ? '#38bdf8' : color
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
  // at max stars there is nothing left to roll — the button just looks cool
  if (props.win) {
    label.text = '😎'
    label.style.fontSize = 64
    label.position.set(0, 0)
    pct.text = ''
    return
  }
  label.text = t('press')
  label.style.fontSize = 26
  label.position.set(0, -8)
  // the talisman bonus rides the roll only, so it shows as its own +N% tag
  pct.text = props.bonus > 0 ? `${props.chance}% +${props.bonus}%` : `${props.chance}%`
  pct.style.fill = props.bonus > 0 ? '#fcd34d' : risky() ? '#fda4af' : '#e2e8f0'
}

function applyDisabled() {
  btn.eventMode = props.disabled ? 'none' : 'static'
  btn.alpha = props.disabled && !props.win ? 0.35 : 1
}

function spawnEmber() {
  if (reducedMotion.matches) return
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
    const palette = props.prestige === 1 && !risky() ? EMBER_COLORS_RARE : EMBER_COLORS
    const hot = Math.random() * 2 + (risky() ? 2 : h * 2)
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
  // disabled: pixi's eventMode is 'none', so a native listener catches the
  // futile tap and answers with the blocked thud (only inside the circle)
  a.canvas.addEventListener('pointerdown', (e) => {
    if (!props.disabled) return
    const r = a.canvas.getBoundingClientRect()
    const x = (e.clientX - r.left) * (SIZE / r.width) - SIZE / 2
    const y = (e.clientY - r.top) * (SIZE / r.height) - SIZE / 2
    if (x * x + y * y <= R * R) play('blocked')
  })

  emberTex = a.renderer.generateTexture(new Graphics().circle(0, 0, 4).fill('#ffffff'))
  // soft blob for steam: stacked translucent circles fake a radial falloff, so
  // puffs read as vapour rather than the hard dots the embers use
  const blob = new Graphics()
  for (let i = 8; i >= 1; i--) blob.circle(0, 0, i * 3).fill({ color: '#ffffff', alpha: 0.06 })
  steamTex = a.renderer.generateTexture(blob)

  fx = new Container()
  btn = new Container()
  // flames live behind the button so their roots are hidden by its face
  flameOuter = new Graphics()
  flameOuter.blendMode = 'add'
  flameCore = new Graphics()
  flameCore.blendMode = 'add'
  halo = new Graphics()
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

  btn.addChild(halo, auraTalisman, shade, rim, label, pct)
  btn.position.set(SIZE / 2, SIZE / 2)
  btn.cursor = 'pointer'
  // flames sit under the particle layer, both under the button itself
  a.stage.addChild(flameOuter, flameCore, fx, btn)

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
    // reduced motion: press response stays, ambient wobble/orbits/embers stop
    const m = reducedMotion.matches ? 0 : 1
    const tremX = (Math.random() - 0.5) * 3 * charge * m
    const tremY = (Math.random() - 0.5) * 3 * charge * m

    const breath = 1 + Math.sin(phase * 1.7) * 0.012 * m
    btn.scale.set(scale * breath)
    btn.position.set(SIZE / 2 + ox + tremX, SIZE / 2 + oy + tremY)
    btn.rotation = Math.sin(phase * 0.9) * 0.02 * m

    // armed card: a slowly rotating, pulsing ring
    auraTalisman.rotation = -phase * 0.8 * m
    auraTalisman.alpha = props.talisman ? 0.4 + Math.sin(phase * 2.6 + 1) * 0.2 * m : 0

    // prestige flair: hue-cycled rim/halo for holo+, orbiting sparkles for prismatic
    if (props.prestige >= 2) {
      hue = (hue + dt * (props.prestige >= 3 ? 2.4 : 0.8)) % 360
      const tint = hslTint(hue)
      rim.tint = tint
      halo.tint = tint
    }
    if (props.prestige >= 3 && m > 0) {
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

    // flames track the risk ladder; the core is shorter and narrower so it
    // reads as the hotter middle of the same fire
    const fl = props.disabled || m === 0 ? 0 : flameLevel()
    // the flames follow the button around but deliberately do NOT take its
    // press scale: the release pop overshoots to ~1.5x, which would fling the
    // tongues past the canvas edge and cut the fire off with a hard straight
    // line. Ambient fire the button pops through also just reads better.
    for (const g of [flameOuter, flameCore]) g.position.set(btn.position.x, btn.position.y)
    drawFlames(flameOuter, fl, '#f97316', 0.72, 0.2, 0.5)
    drawFlames(flameCore, fl, '#fde047', 0.44, 0.11, 0.55)

    // steam builds with the streak, independent of the risk taken
    if (!props.disabled && m > 0) {
      steamAcc += dt * steamLevel() * 0.18
      while (steamAcc >= 1) {
        steamAcc -= 1
        spawnPuff()
      }
    }
    for (let i = puffs.length - 1; i >= 0; i--) {
      const q = puffs[i]
      q.life -= dt
      q.sp.x += (q.vx + Math.sin(q.life * 0.06 + i) * 0.25) * dt
      q.sp.y += q.vy * dt
      const lr = Math.max(q.life / q.max, 0)
      // fade in over the first fifth of the life, then out — puffs should not
      // pop into existence at full opacity
      q.sp.alpha = 0.5 * Math.min(1, (1 - lr) * 5) * lr
      q.sp.scale.set(0.5 + (1 - lr) * 1.4)
      if (q.life <= 0) {
        fx.removeChild(q.sp)
        freePuffs.push(q.sp)
        puffs.splice(i, 1)
      }
    }

    // ember emission scales with how bad the odds are, plus a charging sizzle
    if (!props.disabled) {
      emberAcc += dt * (heat() * (risky() ? 0.9 : 0.55) + charge * 0.4)
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

watch(() => [props.tier, props.risk, props.prestige], () => app && drawButton())
watch(() => [props.chance, props.risk, props.bonus, props.win, lang.value], () => app && syncTexts())
watch(() => [props.disabled, props.win], () => app && applyDisabled())

onBeforeUnmount(() => app?.destroy(true, { children: true }))
</script>

<template>
  <!-- shrinks on short viewports so the core loop still fits without scrolling.
       Pixi maps pointers through getBoundingClientRect, and the press centre
       comes off the host's rect, so CSS scaling keeps both accurate. The
       important modifier is needed because autoDensity writes an inline size. -->
  <div
    ref="host"
    class="h-[min(280px,38dvh)] w-[min(280px,38dvh)] select-none [&>canvas]:!h-full [&>canvas]:!w-full"
  />
</template>
