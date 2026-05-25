import OptionSquare from "@/components/common/optionSquare";
import { useNavigate } from "@tanstack/solid-router";
import Divider from "@/components/common/divider";
import SettingModal from "@/components/modal/settting/settingModal";
import { createEffect } from "solid-js";
import SvgIcon from "@/components/svgIcon";

const Sidebar = () => {
  const navigate = useNavigate();
  let settingModalRef!: HTMLDialogElement;

  createEffect(async () => {
    // if (settingModalRef !== undefined) {
      // settingModalRef?.showModal?.();
    // }
    //  settingModalRef?.showModal?.();
  });
  return (
    <>
      <div class="flex flex-col justify-start items-center space-y-2 px-2 w-16 select-none">
        {/* @ts-ignore */}
        <OptionSquare label="首页" onClick={() => navigate({ to: "/" })}>
          <SvgIcon width={24} height={24} name="home" />
        </OptionSquare>

        <OptionSquare
          label="频道"
          onClick={() => navigate({ to: "/channel", search: { id: 12413 } })}
        >
          <SvgIcon width={24} height={24} name="list-two" fill="none" />
        </OptionSquare>

        <OptionSquare label="设置" onClick={() => settingModalRef.showModal()}>
          <SvgIcon width={24} height={24} name="setting-two" />
        </OptionSquare>
        {/* <OptionSquare label="设置">设置</OptionSquare> */}
        <Divider />

        <div class="flex flex-col flex-1 items-end space-y-2 h-full">
          <OptionSquare label="新会话">+</OptionSquare>
          {/* <OptionSquare>123</OptionSquare> */}
        </div>
      </div>

      <SettingModal
        ref={settingModalRef}
        onClose={() => settingModalRef.close()}
      />
    </>
  );
};
export default Sidebar;
