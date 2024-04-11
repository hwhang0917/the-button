import React from "react";
import Layout from "./components/Layout";
import cn from "./utils/cn";
import useSound from "./hooks/useSound";

function App() {
  const INITIAL_SUCCESS_RATE = 100;
  const DROP_RATE = 5;
  const INITIAL_STAR_COUNT = 1;

  const [successRate, setSuccessRate] = React.useState(INITIAL_SUCCESS_RATE);
  const [disabled, setDisabled] = React.useState(false);
  const [starCount, setStarCount] = React.useState(INITIAL_STAR_COUNT);
  const [highScore, setHighScore] = React.useState(INITIAL_STAR_COUNT);

  const [clickPlay] = useSound("/click.wav");
  const [successPlayLow] = useSound("/success_c.wav");
  const [successPlayMid] = useSound("/success_b.wav");
  const [successPlayHigh] = useSound("/success_a.wav");
  const [failPlay] = useSound("/fail_a.wav");

  const handleClick = async () => {
    setDisabled(true);
    clickPlay();

    setTimeout(() => {
      const isSuccess = Math.random() * 100 < successRate;

      if (isSuccess) {
        setSuccessRate(successRate - DROP_RATE);
        setStarCount((prev) => prev + 1);

        if (starCount >= highScore) {
          setHighScore(starCount + 1);
        }

        if (successRate > 66) {
          successPlayLow();
        } else if (successRate > 33) {
          successPlayMid();
        } else {
          successPlayHigh();
        }
      } else {
        setSuccessRate(INITIAL_SUCCESS_RATE);
        setStarCount(INITIAL_STAR_COUNT);
        failPlay();
      }
    }, 500);

    setTimeout(() => {
      setDisabled(false);
    }, 1000);
  };

  return (
    <Layout>
      <button
        onClick={handleClick}
        disabled={disabled}
        className={cn(
          "text-center text-5xl uppercase font-extrabold",
          "py-2 px-4 text-white uppercase w-full h-16 bg-blue-500",
          "rounded-lg text-3xl transition-all hover:transition-all duration-500",
          "hover:bg-fuchsia-400 disabled:bg-gray-500 disabled:cursor-not-allowed",
        )}
      >
        THE BUTTON
      </button>
      <div className="h-8 w-full flex justify-center flex-wrap py-2">
        {Array.from({ length: starCount })
          .fill(0)
          .map((_, idx) => (
            <span key={idx}>⭐️</span>
          ))}
      </div>
      <h2
        className={cn(
          "text-center text-2xl py-3 font-bold",
          highScore < 5
            ? "text-green-500"
            : highScore < 10
              ? "text-yellow-500"
              : "text-red-300",
        )}
      >
        최고 {highScore}성
      </h2>
      <p className="text-center py-4">
        매번 <strong className="uppercase">the button</strong>을 누를 때 마다,
        성공 확률이 {DROP_RATE}% 만큼 줄어듭니다.
      </p>
    </Layout>
  );
}

export default App;
