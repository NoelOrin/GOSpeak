import type { JSX } from "solid-js/jsx-runtime";
import svgGroup from "@/assets/svg";

interface SvgIconPropsType {
  name: keyof typeof svgGroup;
  width?: number;
  height?: number;
  class?: string;
  stroke?: string;
  fill?: string;
}

const SvgIcon = ({
  name,
  width = 20,
  height = 20,
  stroke,
  fill = "currentColor",
  ...props
}: SvgIconPropsType): JSX.Element => {
  // 获取对应的 SVG 路径

  const svgElement = (
    <svg
      width={width}
      height={height}
      viewBox="0 0 48 48"
      class={props.class}
      stroke={stroke}
      fill={fill}
    >
      {/* @ts-ignore */}
      <use xlink:href={`#${name}`}></use>
    </svg>
  );

  return svgElement;
};

export default SvgIcon;
