import clsx from "clsx";
import { Show } from "solid-js";
import VoiceChatStore from "@/stores/voiceChatStore";
import userStore from "@/stores/userStore";
import Input from "./chat/input";
import Output from "./chat/output";

interface UserBarPropsType {
  class?: string;
}

const UserBar = ({ ...props }: UserBarPropsType) => {
  const {
    data,
    setOutputVolume,
    setInputVolume,
    setIsInputMute,
    setIsOutMute,
  } = VoiceChatStore;

  const user = () => userStore.user();
  const displayName = () => user()?.display_name || user()?.name || "?";
  const initial = () => displayName().charAt(0).toUpperCase();

  return (
    <div
      class={clsx(
        "flex justify-between items-center px-1.5 pb-1.5 dark:text-white select-none",
        props.class,
      )}
    >
      <div class="flex justify-between items-center p-2 border border-color rounded-xl w-full">
        <button class="flex items-center space-x-2" type="button">
          <Show
            when={user()?.avatar}
            fallback={
              <div class="flex justify-center items-center rounded-full size-10 bg-linear-to-br from-primary to-secondary text-primary-content text-base font-bold text-opacity-10">
                {initial()}
              </div>
            }
          >
            <div class="avatar">
              <div class="rounded-full size-10">
                <img src={user()!.avatar} alt={user()!.name} />
              </div>
            </div>
          </Show>
          <div class="flex flex-col items-start">
            <div class="font-bold text-[14px]">{displayName()}</div>
            <div class="text-xs text-base-content/50">{user()?.role ?? ""}</div>
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
