/** Pointer position → the CSS vars driving the poke-holo foil and tilt
 * (.card-tilt / card__shine / card__glare in style.css). The rect must be
 * measured on an untransformed wrapper: measuring the tilted element itself
 * feeds its own rotation back into the pointer math. */
export function tiltVars(clientX: number, clientY: number, r: DOMRect): Record<string, string> {
  const px = (clientX - r.left) / r.width
  const py = (clientY - r.top) / r.height
  return {
    '--rx': `${(px - 0.5) * 24}deg`,
    '--ry': `${(0.5 - py) * 24}deg`,
    // the reference's pointer/background spring vars, same names and ranges
    '--pointer-x': `${px * 100}%`,
    '--pointer-y': `${py * 100}%`,
    '--pointer-from-left': `${px}`,
    '--pointer-from-top': `${py}`,
    '--pointer-from-center': `${Math.min(1, Math.hypot(px - 0.5, py - 0.5) * 2)}`,
    '--background-x': `${37 + px * 26}%`,
    '--background-y': `${33 + py * 34}%`,
    '--card-opacity': '1',
  }
}
