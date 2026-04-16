import { request } from '../../request';
import type { ClientAuth } from '@/typings/api/v1/client-auth';

/**
 * get client captcha
 *
 * this is for terminal users
 */
export function fetchGetClientCaptcha() {
  return request<ClientAuth.CaptchaResult>({
    url: '/client/v1/auth/captcha',
    method: 'get'
  });
}

/**
 * send verification code (SMS/Email)
 *
 * @param data send code request
 */
export function fetchSendVerifyCode(data: ClientAuth.SendCodeReq) {
  return request({
    url: '/client/v1/auth/send-code',
    method: 'post',
    data
  });
}

/**
 * get verify config
 *
 * @param scene register, reset_password
 */
export function fetchGetVerifyConfig(scene: string) {
  return request<ClientAuth.VerifyConfig>({
    url: '/client/v1/auth/verify-config',
    method: 'get',
    params: { scene }
  });
}
