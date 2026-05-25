import SvgIcon from "../svgIcon";

interface OutputPropsType {
  name?: string;
  volume: () => number;
  isMute: () => boolean;
  onChange: (volume: number) => void;
  onCheck: (isMute: boolean) => void;
}

const Input = ({ volume, isMute, onChange, onCheck,...props }: OutputPropsType) => {
  return (
    <div class="dropdown-top dropdown dropdown-hover">
      <div
        tabIndex={0}
        role="button"
        class="flex justify-center items-center rounded-3xl dark:text-white pointer-events-auto"
      >
        <label class="swap swap-flip">
          <input
            type="checkbox"
            name={props.name || "input"}
            checked={isMute()}
            onInput={(e) => onCheck(e.target.checked)}
          />

          {/* microphone on icon */}
          <SvgIcon name="microphone-on" class="swap-on" />

          {/* microphone off icon */}
          <SvgIcon name="microphone-off" class="swap-off" />
        </label>
      </div>

      <div
        tabIndex="-1"
        class="z-1 px-2 pb-3 -translate-x-[30%] dropdown-content menu"
      >
        <div
          data-tip={volume()}
          class="flex flex-col justify-center items-center bg-base-100 shadow-sm border rounded-box w-6 h-25 tooltip"
        >
          <input
            type="range"
            min="0"
            max="100"
            value={volume()}
            onInput={(e) => onChange(Number(e.target.value))}
            class="w-22 rotate-270 origin-right -translate-x-1/2 -translate-y-11 range range-xs"
          />
        </div>
      </div>
    </div>
  );
};
export default Input;
