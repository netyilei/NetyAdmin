import type { ClientUser } from '@/typings/api/v1/client-user';
import { request } from '../../request';

/**
 * user register
 *
 * @param data register request
 */
export function fetchClientRegister(data: ClientUser.RegisterReq) {
  return request<{ id: string }>({
    url: '/client/v1/user/register',
    method: 'post',
    data
  });
}

/**
 * user reset password
 *
 * @param data reset password request
 */
export function fetchClientResetPassword(data: ClientUser.ResetPasswordReq) {
  return request({
    url: '/client/v1/user/reset-password',
    method: 'post',
    data
  });
}

/**
 * user login (client side)
 *
 * @param data login request
 */
export function fetchClientLogin(data: any) {
  return request<ClientUser.LoginToken>({
    url: '/client/v1/user/login',
    method: 'post',
    data
  });
}
