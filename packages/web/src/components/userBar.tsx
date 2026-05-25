import clsx from "clsx";
import VoiceChatStore from "@/stores/voiceChatStore";
import Input from "./chat/input";
import Output from "./chat/output";
import Avatar from "./common/avatar";

interface UserBarPropsType {
  class?: string;
}

const UserBar = ({ ...props }: UserBarPropsType) => {
  const { data, setOutputVolume, setInputVolume, setIsInputMute, setIsOutMute } =
    VoiceChatStore;
  return (
    <div
      class={clsx(
        "flex justify-between items-center px-1.5 pb-1.5 dark:text-white select-none",
        props.class
      )}
    >
      <div class="flex justify-between items-center p-2 border border-color rounded-xl w-full">
        <button class="flex items-center space-x-2" type="button">
          <Avatar
            avatarURL={
              "https://img.daisyui.com/images/profile/demo/distracted1@192.webp"
            }
            class="size-10"
          />
          <div class="flex flex-col items-start">
            <div class="font-bold text-[14px]">张三</div>
            <div class="text-xs">1234567890</div>
          </div>
        </button>

        <div class="flex space-x-3">
          <Output
            volume={() => data.outputVolume}
            onChange={setOutputVolume}
            isMute={() => data.isOutMute}
            onCheck={setIsOutMute}
          />
          <Input
            volume={() => data.inputVolume}
            onChange={setInputVolume}
            isMute={() => data.isInputMute}
            onCheck={setIsInputMute}
          />
        </div>
      </div>
    </div>
  );
};

export default UserBar;
