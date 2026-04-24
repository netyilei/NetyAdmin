export namespace ClientUser {
  interface RegisterReq {
    username: string;
    password: string;
    nickname: string;
    phone?: string;
    email?: string;
    code: string;
  }

  interface ResetPasswordReq {
    target: string;
    code: string;
    newPassword: string;
  }

  interface LoginToken {
    accessToken: string;
    refreshToken: string;
    expiresIn: number;
  }

  interface UserInfo {
    id: string;
    userName: string;
    nickName: string;
    avatar: string;
    phone: string;
    email: string;
    gender: string;
    status: string;
    lastLoginAt?: string;
  }

  type ClientUserSearchParams = CommonType.RecordNullable<
    {
      username: string;
      nickname: string;
      phone: string;
      email: string;
      gender: string | null;
      status: string | null;
    } & import('@/typings/api/v1/common').Common.CommonSearchParams
  >;

  type ClientUserList = import('@/typings/api/v1/common').Common.PaginatingQueryRecord<UserInfo>;
}
