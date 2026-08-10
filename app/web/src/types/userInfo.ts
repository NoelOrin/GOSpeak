export type UserInfo = {
	id: string;
} & LocalUserInfo;

export type LocalUserInfo = {
	name: string;
	avatar: string;
};
