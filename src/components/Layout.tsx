import React from "react";
import { Github, RotateCcw } from "lucide-react";
import Confetti from "react-confetti";
import { useWindowSize } from "usehooks-ts";
import { computeRank, useGame, useHighScore } from "../stores/game";
import useSound from "../hooks/useSound";
import cn from "../utils/cn";
import { BASE } from "../constants";

export default function Layout(props: { children?: React.ReactNode }) {
  const { width, height } = useWindowSize();
  const { highStarCount } = useHighScore();
  const { starCount } = useGame();
  const { confettiCount, resetGame } = useGame();
  const [playSwitch] = useSound(`${BASE}switch.wav`);

  return (
    <React.Fragment>
      <header
        className={cn(
          "fixed top-0 z-10 w-full py-4",
          "flex justify-around items-center gap-2",
          "text-white bg-black/75",
          "text-xl",
        )}
      >
        <a
          href="https://github.com/hwhang0917/the-button"
          target="_blank"
          className="hover:scale-110 transition-transform"
          onClick={() => playSwitch()}
        >
          <Github />
        </a>
        <div>
          <h2>🌟 최고 기록: {highStarCount} 성 🌟</h2>
          <h2 className="text-center uppercase">
            {computeRank(highStarCount)}
          </h2>
        </div>
        <button
          className={cn(
            "active:-rotate-90 transition-transform",
            "hover:scale-110",
          )}
          onClick={() => {
            playSwitch();
            resetGame();
          }}
        >
          <RotateCcw />
        </button>
      </header>
      <Confetti width={width} height={height} numberOfPieces={confettiCount} />
      <section
        className={cn(
          "absolute inset-0",
          "w-screen h-screen",
          "bg-[url('/the-button/wallpaper.jpg')] bg-cover",
          "brightness-50",
        )}
      />
      <main className={cn("w-screen h-screen grid place-items-center")}>
        {props.children}
      </main>
      <footer
        className={cn(
          "fixed bottom-0 w-full py-4 px-16",
          "text-white bg-black/75",
          "text-xl",
        )}
      >
        <h2 className="w-full text-center uppercase">
          현재 랭크: {computeRank(starCount)}
        </h2>
        <ul className="h-10 flex justify-center items-center">
          {Array.from({ length: starCount })
            .fill(0)
            .map((_, i) => (
              <li key={i}>
                <a
                  href="https://www.flaticon.com/free-icons/star"
                  title="star icons created by Freepik - Flaticon"
                >
                  <img src={`${BASE}star.png`} alt="🌟" className="w-5 h-5" />
                </a>
              </li>
            ))}
        </ul>
      </footer>
    </React.Fragment>
  );
}
