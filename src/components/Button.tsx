import React from "react";
import useSound from "../hooks/useSound";
import { computeRank, useGame, useHighScore } from "../stores/game";
import {
  BASE,
  DECREASE_RATE,
  DEFAULT_DELAY,
  WINNING_STAR_COUNT,
} from "../constants";
import { cn, vibrate } from "../utils";

export default function Button() {
  const {
    successRate,
    starCount,
    btnStage,
    toggleConfetti,
    increaseStarCount,
    decreaseSuccessRate,
    resetGame,
    setButtonStage,
  } = useGame();
  const { highStarCount, setHighStarCount } = useHighScore();
  const [isFail, setIsFail] = React.useState(false);
  const [playClick] = useSound(`${BASE}click.wav`);
  const [playMouseover] = useSound(`${BASE}mouseover.wav`);
  const [playWin] = useSound(`${BASE}win.wav`);

  const [playSuccess, { stop: stopSuccess }] = useSound(
    `${BASE}success_${computeRank(starCount)}.wav`,
  );
  const [playFail] = useSound(`${BASE}fail.wav`);
  const disabled = React.useMemo(
    () => btnStage === "loading" || btnStage === "result" || btnStage === "win",
    [btnStage],
  );

  const computeSuccess = async () => {
    return new Promise((resolve) => {
      setTimeout(() => {
        const isSuccess = Math.random() < successRate / 100;
        resolve(isSuccess);
      }, DEFAULT_DELAY);
    });
  };
  const throwConfetti = async () => {
    toggleConfetti();
    return new Promise((resolve) => {
      setTimeout(() => {
        toggleConfetti();
        resolve(true);
      }, DEFAULT_DELAY);
    });
  };

  const handleClick = async () => {
    vibrate(100);
    setButtonStage("loading");
    playClick();
    stopSuccess();
    const isSuccess = await computeSuccess();
    setButtonStage("result");

    if (isSuccess) {
      playSuccess();
      increaseStarCount();
      decreaseSuccessRate(DECREASE_RATE);

      // WIN
      if (starCount + 1 >= WINNING_STAR_COUNT) {
        vibrate(1_000);
        stopSuccess();
        setButtonStage("win");
        playWin();
        toggleConfetti();
        setHighStarCount(20);
        return;
      }

      if (starCount >= highStarCount) {
        setHighStarCount(starCount + 1);
      }
      await throwConfetti();
    } else {
      playFail();
      setIsFail(true);
      setTimeout(() => {
        setIsFail(false);
      }, 1_000);
      resetGame();
    }

    setButtonStage("standby");
  };

  return (
    <React.Fragment>
      <button
        className={cn(
          "glowing-btn uppercase",
          "w-1/2 h-14 md:w-1/3 md:h-16",
          btnStage === "loading" ? "cursor-wait" : "",
          btnStage === "win" ? "cursor-default" : "",
        )}
        disabled={disabled}
        onClick={handleClick}
        onMouseOver={() => playMouseover()}
      >
        {isFail
          ? "😭 실패 😭"
          : btnStage === "loading"
            ? "분석중"
            : btnStage === "result"
              ? "🌟 성공 🌟"
              : btnStage === "win"
                ? "🎉 YOU WIN 🎉"
                : `THE BUTTON (${successRate} %)`}
      </button>
    </React.Fragment>
  );
}
