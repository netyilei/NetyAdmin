/**
 * namespace Auth
 *
 * backend api module: "auth"
 */
export namespace Auth {
  interface LoginToken {
    token: string;
    refreshToken: string;
  }

  interface UserInfo {
    userId: string;
    userName: string;
    roles: string[];
    buttons: string[];
  }

  type UserGender = '1' | '2';

  interface Profile {
    id: number;
    userName: string;
    nickName: string;
    userPhone: string;
    userEmail: string;
    userGender: UserGender;
    status: string;
    createTime: string;
  }

  interface UpdateProfileParams {
    nickName: string;
    userPhone: string;
    userEmail: string;
    userGender: UserGender;
  }

  interface ChangePasswordParams {
    oldPassword: string;
    newPassword: string;
  }
}
