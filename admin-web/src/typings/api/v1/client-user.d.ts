export namespace ClientUser {
  /** user registration request */
  interface RegisterReq {
    username: string;
    password: string;
    nickname: string;
    phone?: string;
    email?: string;
    code: string;
  }

  /** user reset password request */
  interface ResetPasswordReq {
    target: string;
    code: string;
    newPassword: string;
  }

  /** login token */
  interface LoginToken {
    accessToken: string;
    refreshToken: string;
    expiresIn: number;
  }

  /** user info */
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
}
