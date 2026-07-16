/**
 * The app's entire motion vocabulary. Exactly 3 durations, one easing for
 * entrances — no spring/bounce/scale-pop, no per-page ad hoc values.
 * Previously found: 5 different durations (0.14s-0.6s) and 3 different
 * stagger deltas across pages with no shared source.
 */
export const DUR = {
  micro: 0.12,   // hover/press feedback
  default: 0.18, // most transitions
  enter: 0.24,   // panels, sheets, dialogs, command palette
} as const;

export const EASE_ENTER: [number, number, number, number] = [0.16, 1, 0.3, 1];

/** Standard list/card entrance — replaces the copy-pasted
 * `initial={{opacity:0,y:5}}` blocks with varying durations. */
export const fadeUp = {
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: DUR.enter, ease: EASE_ENTER },
};

/** Stagger delta for lists — one constant, capped so long lists don't feel
 * sluggish (previously ranged 0.016-0.06 inconsistently). */
export const STAGGER_STEP = 0.03;
export const STAGGER_MAX = 0.3;

export function staggerDelay(index: number) {
  return Math.min(index * STAGGER_STEP, STAGGER_MAX);
}

/** Simple fade for modals/dialogs backdrop + panel. */
export const fadeScale = {
  initial: { opacity: 0, scale: 0.98 },
  animate: { opacity: 1, scale: 1 },
  exit: { opacity: 0, scale: 0.98 },
  transition: { duration: DUR.enter, ease: EASE_ENTER },
};
