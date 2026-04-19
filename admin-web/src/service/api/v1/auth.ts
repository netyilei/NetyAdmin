import type { Auth } from '@/typings/api/v1/auth';
import { request } from '../../request';

export function fetchLogin(params: Auth.LoginReq) {
  return request<Auth.LoginToken>({
    url: '/admin/v1/auth/login',
    method: 'post',
    data: params
  });
}

/** 获取验证码 */
export function fetchGetCaptcha() {
  return request<{ captchaId: string; captchaImg: string }>({
    url: '/admin/v1/common/captcha',
    method: 'get'
  });
}

export function fetchGetUserInfo() {
  return request<Auth.UserInfo>({ url: '/admin/v1/auth/getUserInfo' });
}

export function fetchGetProfile() {
  return request<Auth.Profile>({ url: '/admin/v1/auth/profile' });
}

export function fetchUpdateProfile(data: Auth.UpdateProfileParams) {
  return request({
    url: '/admin/v1/auth/profile',
    method: 'put',
    data
  });
}

export function fetchChangePassword(data: Auth.ChangePasswordParams) {
  return request({
    url: '/admin/v1/auth/changePassword',
    method: 'post',
    data
  });
}

export function fetchRefreshToken(refreshToken: string) {
  return request<Auth.LoginToken>({
    url: '/admin/v1/auth/refreshToken',
    method: 'post',
    data: {
      refreshToken
    }
  });
}
