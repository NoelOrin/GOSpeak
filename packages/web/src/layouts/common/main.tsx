import type { JSX } from "solid-js/jsx-runtime";



const Main = ({ children }: { children: JSX.Element; }) => {


  return (
    <div class="flex-1 w-full h-full select-none">
          <div class="relative flex-1 bg-black/3.5 dark:bg-white/10 h-full">
            {children}
          </div>
    </div>
  );
};

export default Main;