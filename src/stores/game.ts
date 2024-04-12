import { create } from "zustand";
import { persist } from "zustand/middleware";
import { INITIAL_SUCCESS_RATE, SUCCESS_CONFETTI_COUNT } from "../constants";

type ButtonStage = "standby" | "loading" | "result" | "win";

interface GameStore {
  starCount: number;
  successRate: number;
  confettiCount: number;
  btnStage: ButtonStage;
  increaseStarCount: () => void;
  toggleConfetti: () => void;
  decreaseSuccessRate: (decreaseRate: number) => void;
  setButtonStage: (btnStage: ButtonStage) => void;
  resetGame: () => void;
}

interface HighScoreStore {
  highStarCount: number;
  setHighStarCount: (highStarCount: number) => void;
}

export const useHighScore = create(
  persist<HighScoreStore>(
    (set) => ({
      highStarCount: 0,
      setHighStarCount: (highStarCount: number) => set({ highStarCount }),
    }),
    {
      name: "high-score",
    },
  ),
);

export const useGame = create<GameStore>((set) => ({
  starCount: 0,
  successRate: INITIAL_SUCCESS_RATE,
  confettiCount: 0,
  btnStage: "standby",
  increaseStarCount: () => set((state) => ({ starCount: state.starCount + 1 })),
  setButtonStage: (btnStage: ButtonStage) => set({ btnStage }),
  toggleConfetti: () =>
    set((state) =>
      state.confettiCount === 0
        ? { confettiCount: SUCCESS_CONFETTI_COUNT }
        : { confettiCount: 0 },
    ),
  decreaseSuccessRate: (decreaseRate: number) =>
    set((state) => ({ successRate: state.successRate - decreaseRate })),
  resetGame: () =>
    set({
      starCount: 0,
      successRate: INITIAL_SUCCESS_RATE,
      confettiCount: 0,
      btnStage: "standby",
    }),
}));

export const computeRank = (starCount: number) => {
  if (starCount < 3) return "unrank";
  else if (starCount < 5) return "bronze";
  else if (starCount < 7) return "silver";
  else if (starCount < 10) return "gold";
  else if (starCount < 15) return "platinum";
  else return "diamond";
};
