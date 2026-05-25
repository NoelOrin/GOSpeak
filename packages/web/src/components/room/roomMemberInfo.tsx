import type { RoomMemberInfoType } from "@/types/room";
import Avatar from "../common/avatar";

interface MemberInfoPropsType {
  memberInfo: RoomMemberInfoType;
}

const MemberInfo = ({ memberInfo }: MemberInfoPropsType) => {
  return (
    <div class="flex flex-col space-y-1 ml-4 px-2 border-0 h-fit select-none btn-xs btn btn-ghost">
      <div class="flex justify-between items-center py-1 w-full font-semibold text-[12px]">
        <div class="flex items-center space-x-2 w-full">
          <Avatar class="size-5.5" avatarURL={memberInfo.avatar} />
          <span class="text-[12px]">{memberInfo.name}</span>
        </div>
      </div>
    </div>
  );
};
export default MemberInfo;
