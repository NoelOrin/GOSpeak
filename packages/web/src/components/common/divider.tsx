import clsx from "clsx";
import type { JSX } from "solid-js/jsx-runtime";
interface DividerProps {
  class?: string;
  children?: string | JSX.Element;
}
const Divider = ({ ...props }: DividerProps) => {
  return <div class={clsx("divider", props.class)}>{props?.children}</div>;
};
export default Divider;
