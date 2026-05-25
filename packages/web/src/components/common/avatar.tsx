interface AvatarProps {
  avatarURL: string;
  class?: string;
}

const Avatar = ({ avatarURL, ...props }: AvatarProps) => {
  return (
    <div class="avatar">
      <div class={`rounded-full ${props.class || "size-13"}`}>
        <img src={avatarURL} alt={avatarURL} />
      </div>
    </div>
  );
};

export default Avatar;
