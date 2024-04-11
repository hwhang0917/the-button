import React from "react";
import useSound from "../hooks/useSound";
import { computeRank, useGame, useHighScore } from "../stores/game";
import { DECREASE_RATE, DEFAULT_DELAY } from "../constants";
import cn from "../utils/cn";

export default function Button() {
  const {
    successRate,
    starCount,
    toggleConfetti,
    increaseStarCount,
    decreaseSuccessRate,
    resetGame,
  } = useGame();
  const { highStarCount, setHighStarCount } = useHighScore();
  const [playClick] = useSound("/click.wav");
  const [playMouseover] = useSound("/mouseover.wav");
  const [playAchievement] = useSound("/achievement.wav");

  const [playSuccess] = useSound(`/success_${computeRank(starCount)}.wav`);
  const [playFail] = useSound("/fail.wav");
  const [btnStage, setBtnStage] = React.useState<
    "standby" | "loading" | "result" | "win"
  >("standby");
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
    setBtnStage("loading");
    playClick();
    const isSuccess = await computeSuccess();
    setBtnStage("result");

    if (isSuccess) {
      playSuccess();
      increaseStarCount();
      decreaseSuccessRate(DECREASE_RATE);

      if (successRate - DECREASE_RATE <= 0) {
        setBtnStage("win");
        playAchievement();
        toggleConfetti();
        return;
      }

      if (starCount >= highStarCount) {
        setHighStarCount(starCount + 1);
      }
      await throwConfetti();
    } else {
      playFail();
      resetGame();
    }

    setBtnStage("standby");
  };

  return (
    <React.Fragment>
      <button
        className={cn(
          "glowing-btn uppercase w-96",
          disabled ? "cursor-wait" : "",
        )}
        disabled={disabled}
        onClick={handleClick}
        onMouseOver={() => playMouseover()}
      >
        {btnStage === "loading"
          ? "로딩중..."
          : btnStage === "win"
            ? "🎉 YOU WIN 🎉"
            : `the button (${successRate} %)`}
      </button>
    </React.Fragment>
  );
}
