import { BASE } from "../constants";

export default function Win() {
  return (
    <div className="fixed z-50 top-1/2 right-1/2 translate-x-1/2 -translate-y-48 md:-translate-y-56">
      <img src={`${BASE}stich.gif`} />
    </div>
  );
}
