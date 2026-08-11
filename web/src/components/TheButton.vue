<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Application, Container, Graphics, Sprite, Text, Texture } from 'pixi.js'
import { TIER_COLORS, type Tier } from '../tiers'
import { t, lang } from '../i18n'
import { play } from '../audio'

const props = defineProps<{
  tier: Tier
  chance: number
  risky: boolean
  disabled: boolean
}>()
const emit = defineEmits<{ press: [center: { x: number; y: number }] }>()

const host = ref<HTMLDivElement | null>(null)

const SIZE = 280
const R = 88

let app: Application | null = null
let btn: Container
let fx: Container
let halo: Graphics
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

// 0..1: embers start under ~60% odds and rage as the odds shrink
function heat(): number {
  return Math.max(0, Math.min(1, (60 - props.chance) / 55))
}

function drawButton() {
  const color = TIER_COLORS[props.tier]
  const edge = props.risky ? '#f43f5e' : color
  halo.clear()
  for (let i = 3; i >= 1; i--) {
    halo.circle(0, 0, R + i * 13).fill({ color: edge, alpha: 0.045 * (4 - i) })
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
  pct.text = `${props.chance}%`
  pct.style.fill = props.risky ? '#fda4af' : '#e2e8f0'
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
  const hot = Math.random() * 2 + (props.risky ? 2 : h * 2)
  sp.tint = EMBER_COLORS[Math.min(EMBER_COLORS.length - 1, Math.floor(hot))]
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

  emberTex = a.renderer.generateTexture(new Graphics().circle(0, 0, 4).fill('#ffffff'))

  fx = new Container()
  btn = new Container()
  halo = new Graphics()
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
    style: { fontFamily: 'ui-monospace, monospace', fontSize: 16, fontWeight: '700', fill: '#e2e8f0' },
  })
  pct.anchor.set(0.5)
  pct.position.set(0, 24)

  btn.addChild(halo, shade, rim, label, pct)
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
    targetScale = 0.82
    play('click')
    vy += 1.5
  })
  const release = () => {
    targetScale = 1
    pressed = false
  }
  btn.on('pointerupoutside', release)
  btn.on('pointerup', () => {
    if (!pressed) return
    release()
    vScale += 0.06 // springy overshoot on release
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

    const breath = 1 + Math.sin(phase * 1.7) * 0.012
    btn.scale.set(scale * breath)
    btn.position.set(SIZE / 2 + ox, SIZE / 2 + oy)
    btn.rotation = Math.sin(phase * 0.9) * 0.02

    // ember emission scales with how bad the odds are
    if (!props.disabled) {
      emberAcc += dt * heat() * (props.risky ? 0.9 : 0.55)
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

watch(() => [props.tier, props.risky], () => app && drawButton())
watch(() => [props.chance, props.risky, lang.value], () => app && syncTexts())
watch(() => props.disabled, () => app && applyDisabled())

onBeforeUnmount(() => app?.destroy(true, { children: true }))
</script>

<template>
  <div ref="host" class="h-[280px] w-[280px] select-none" />
</template>
