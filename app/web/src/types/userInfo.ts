export type UserInfo = {
	id: string;
} & LocalUserInfo;

export type LocalUserInfo = {
	name: string;
	avatar: string;
};

export type Token = {
	accessToken: string;
	refreshToken: string;
};
