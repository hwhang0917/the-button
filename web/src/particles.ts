interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  life: number
  maxLife: number
  size: number
  color: string
  gravity: number
}

let canvas: HTMLCanvasElement | null = null
let ctx: CanvasRenderingContext2D | null = null
const particles: Particle[] = []
let running = false

function ensureCanvas() {
  if (canvas) return
  canvas = document.createElement('canvas')
  canvas.style.cssText = 'position:fixed;inset:0;pointer-events:none;z-index:50'
  document.body.appendChild(canvas)
  const resize = () => {
    canvas!.width = window.innerWidth
    canvas!.height = window.innerHeight
  }
  resize()
  window.addEventListener('resize', resize)
  ctx = canvas.getContext('2d')
}

function loop() {
  if (!ctx || !canvas) return
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  for (let i = particles.length - 1; i >= 0; i--) {
    const p = particles[i]
    p.x += p.vx
    p.y += p.vy
    p.vy += p.gravity
    p.life--
    if (p.life <= 0) {
      particles.splice(i, 1)
      continue
    }
    ctx.globalAlpha = p.life / p.maxLife
    ctx.fillStyle = p.color
    ctx.fillRect(p.x, p.y, p.size, p.size)
  }
  ctx.globalAlpha = 1
  if (particles.length > 0) {
    requestAnimationFrame(loop)
  } else {
    running = false
  }
}

function pump() {
  if (!running) {
    running = true
    requestAnimationFrame(loop)
  }
}

export function burst(x: number, y: number, colors: string[], count = 60) {
  ensureCanvas()
  for (let i = 0; i < count; i++) {
    const angle = Math.random() * Math.PI * 2
    const speed = 2 + Math.random() * 7
    particles.push({
      x,
      y,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed - 2,
      life: 40 + Math.random() * 30,
      maxLife: 70,
      size: 3 + Math.random() * 4,
      color: colors[Math.floor(Math.random() * colors.length)],
      gravity: 0.12,
    })
  }
  pump()
}

export function confetti() {
  ensureCanvas()
  const colors = ['#f43f5e', '#facc15', '#4ade80', '#38bdf8', '#a78bfa', '#fb923c']
  for (let i = 0; i < 220; i++) {
    particles.push({
      x: Math.random() * window.innerWidth,
      y: -20 - Math.random() * window.innerHeight * 0.5,
      vx: (Math.random() - 0.5) * 3,
      vy: 2 + Math.random() * 3,
      life: 160 + Math.random() * 80,
      maxLife: 240,
      size: 4 + Math.random() * 5,
      color: colors[Math.floor(Math.random() * colors.length)],
      gravity: 0.03,
    })
  }
  pump()
}
