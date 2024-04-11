import React from "react";
import cn from "../utils/cn";

export default function Layout(props: { children?: React.ReactNode }) {
  return (
    <section
      className={cn(
        "w-screen h-screen",
        "bg-gradient-to-r from-slate-900 to-slate-800",
        "grid place-items-center",
      )}
    >
      <main
        className={cn(
          "py-8 px-24 text-white",
          "isolate rounded-xl bg-white/20 shadow-lg ring-1 ring-black/5",
        )}
      >
        {props.children}
      </main>
    </section>
  );
}
