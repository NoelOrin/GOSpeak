export type UserInfo = {
	id: string;
} & LocalUserInfo &
	keyof Token;

export type LocalUserInfo = {
	name: string;
	avatar: string;
};

export type Token = {
	accessToken: string;
	refreshToken: string;
};
