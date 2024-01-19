import { useState } from "react";

function App() {
    const [count, setCount] = useState(0);

    return (
        <main>
            <h1>Vite + React</h1>
            <div className="grid gap-4">
                <button onClick={() => setCount((count) => count + 1)}>
                    count is {count}
                </button>
                <p className="text-red-500 flex">
                    Edit <code>src/App.tsx</code> and save to test HMR
                </p>
            </div>
            <p className="text-sm">
                Click on the Vite and React logos to learn more
            </p>
        </main>
    );
}

export default App;
