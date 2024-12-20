import React from "react";
import { Github, RotateCcw } from "lucide-react";
import Confetti from "react-confetti";
import { useWindowSize } from "usehooks-ts";
import Win from "./Win";
import { computeRank, useGame, useHighScore } from "../stores/game";
import useSound from "../hooks/useSound";
import { cn, vibrate } from "../utils";
import { BASE } from "../constants";

export default function Layout(props: { children?: React.ReactNode }) {
  const { width, height } = useWindowSize();
  const { highStarCount } = useHighScore();
  const { starCount, btnStage } = useGame();
  const { confettiCount, resetGame } = useGame();
  const [playSwitch] = useSound(`${BASE}switch.wav`);

  const rankColor = React.useCallback((c: number) => {
    const rank = computeRank(c);
    if (rank === "unrank") return "text-white/30";
    else if (rank === "bronze") return "text-[#cd7f32]";
    else if (rank === "silver") return "text-[#e5e4e2]";
    else if (rank === "gold") return "text-[#d4af37]";
    else if (rank === "platinum") return "text-teal-500";
    else if (rank === "diamond") return "text-blue-500";
  }, []);

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
          href="https://github.com/hwhang0917"
          target="_blank"
          className="hover:scale-110 transition-transform"
          onClick={() => {
            vibrate(10);
            playSwitch();
          }}
        >
          <Github />
        </a>
        <div>
          <h2>🌟 최고 기록: {highStarCount} 성 🌟</h2>
          <h2 className={cn("text-center uppercase", rankColor(highStarCount))}>
            {computeRank(highStarCount)}
          </h2>
        </div>
        <button
          className={cn(
            "active:-rotate-90 transition-transform",
            "hover:scale-110",
          )}
          onClick={() => {
            vibrate(10);
            playSwitch();
            resetGame();
          }}
        >
          <RotateCcw />
        </button>
      </header>
      <Confetti width={width} height={height} numberOfPieces={confettiCount} />
      {btnStage === "win" && <Win />}
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
        <ul className="min-h-20 flex justify-center items-center flex-wrap py-4">
          {Array.from({ length: starCount })
            .fill(0)
            .map((_, i) => (
              <li key={i}>
                <a
                  href="https://www.flaticon.com/free-icons/star"
                  title="star icons created by Freepik - Flaticon"
                >
                  <img
                    src={`${BASE}star.png`}
                    alt="🌟"
                    className="w-8 h-8 md:w-10 md:h-10"
                  />
                </a>
              </li>
            ))}
        </ul>
        <h2 className={cn("w-full text-center uppercase")}>
          현재 랭크
          <br />
          <span className={rankColor(starCount)}>{computeRank(starCount)}</span>
        </h2>
        <p className="text-xs text-gray-500 text-center uppercase pt-4">
          Copyright &copy; {new Date().getFullYear()}{" "}
          <a
            href="https://runfridge.dev"
            className="underline-offset-2 underline"
          >
            RunFridge
          </a>
          . All rights reserved.
        </p>
      </footer>
    </React.Fragment>
  );
}
