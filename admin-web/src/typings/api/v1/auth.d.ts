export namespace Auth {
  interface LoginReq {
    username: string;
    password: string;
    captchaId?: string;
    captchaValue?: string;
  }

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

  type UserGender = import('@/typings/api/v1/system-manage').SystemManage.AdminGender;

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
    userGender: NonNullable<UserGender>;
  }

  interface ChangePasswordParams {
    oldPassword: string;
    newPassword: string;
  }
}
