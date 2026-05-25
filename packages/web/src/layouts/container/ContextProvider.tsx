// import { createContext, createEffect, createSignal, type JSX } from "solid-js";
import { createContext, createSignal, type JSX } from "solid-js";

export const ThemeContext = createContext();

const ContextProvider = ({ children }: { children: JSX.Element }) => {
  const [theme, _setTheme] = createSignal("light");

  const setTheme = (theme: string) => {
    if (theme === "light" || theme === "dark") {
      _setTheme(theme);
    } else {
      console.error(`Invalid theme: ${theme}`);
    }
  };
  const state = {
    theme,
    setTheme,
  };

  // createEffect(() => {
  // 	console.log(state);
  // });

  return (
    <ThemeContext.Provider value={state}>{children}</ThemeContext.Provider>
  );
};
export default ContextProvider;