import type { ClientAuth } from '@/typings/api/v1/client-auth';
import { request } from '../../request';

/**
 * get client captcha image
 */
export function fetchGetClientCaptcha() {
  return request<ClientAuth.CaptchaResult>({
    url: '/client/v1/auth/captcha',
    method: 'get'
  });
}

/**
 * get scene config (merged captcha + verify config)
 *
 * @param scene login, register, reset_password
 */
export function fetchGetSceneConfig(scene: string) {
  return request<ClientAuth.SceneConfig>({
    url: '/client/v1/auth/scene-config',
    method: 'get',
    params: { scene }
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
