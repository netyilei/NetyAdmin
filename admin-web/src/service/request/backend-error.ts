import { $t } from '@/locales';

export const BackendErrorCode = {
  invalidParams: '100001',
  unauthorized: '100002',
  forbidden: '100003',
  notFound: '100004',
  internalError: '100005',
  tooManyRequest: '100006',
  badRequest: '100007',
  alreadyExists: '100008',
  userNotFound: '101001',
  userDisabled: '101002',
  passwordWrong: '101003',
  userAlreadyExists: '101004',
  tokenExpired: '101005',
  tokenInvalid: '101006',
  oldPasswordWrong: '101007',
  roleNotFound: '102001',
  roleInUse: '102002',
  roleAlreadyExists: '102003',
  roleCodeDuplicate: '102004',
  cannotDeleteSuper: '102005',
  cannotModifySuper: '102006',
  menuNotFound: '103001',
  menuHasChildren: '103002',
  menuAlreadyExists: '103003',
  menuRouteDuplicate: '103004',
  buttonNotFound: '104001',
  buttonAlreadyExists: '104002',
  buttonCodeDuplicate: '104003',
  apiNotFound: '105001',
  apiAlreadyExists: '105002',
  apiPathDuplicate: '105003'
} as const;

const backendErrorI18nKeyMap: Record<string, string> = {
  [BackendErrorCode.invalidParams]: 'request.backend.invalidParams',
  [BackendErrorCode.unauthorized]: 'request.backend.unauthorized',
  [BackendErrorCode.forbidden]: 'request.backend.forbidden',
  [BackendErrorCode.notFound]: 'request.backend.notFound',
  [BackendErrorCode.internalError]: 'request.backend.internalError',
  [BackendErrorCode.tooManyRequest]: 'request.backend.tooManyRequest',
  [BackendErrorCode.badRequest]: 'request.backend.badRequest',
  [BackendErrorCode.alreadyExists]: 'request.backend.alreadyExists',
  [BackendErrorCode.userNotFound]: 'request.backend.userNotFound',
  [BackendErrorCode.userDisabled]: 'request.backend.userDisabled',
  [BackendErrorCode.passwordWrong]: 'request.backend.passwordWrong',
  [BackendErrorCode.userAlreadyExists]: 'request.backend.userAlreadyExists',
  [BackendErrorCode.tokenExpired]: 'request.backend.tokenExpired',
  [BackendErrorCode.tokenInvalid]: 'request.backend.tokenInvalid',
  [BackendErrorCode.oldPasswordWrong]: 'request.backend.oldPasswordWrong',
  [BackendErrorCode.roleNotFound]: 'request.backend.roleNotFound',
  [BackendErrorCode.roleInUse]: 'request.backend.roleInUse',
  [BackendErrorCode.roleAlreadyExists]: 'request.backend.roleAlreadyExists',
  [BackendErrorCode.roleCodeDuplicate]: 'request.backend.roleCodeDuplicate',
  [BackendErrorCode.cannotDeleteSuper]: 'request.backend.cannotDeleteSuper',
  [BackendErrorCode.cannotModifySuper]: 'request.backend.cannotModifySuper',
  [BackendErrorCode.menuNotFound]: 'request.backend.menuNotFound',
  [BackendErrorCode.menuHasChildren]: 'request.backend.menuHasChildren',
  [BackendErrorCode.menuAlreadyExists]: 'request.backend.menuAlreadyExists',
  [BackendErrorCode.menuRouteDuplicate]: 'request.backend.menuRouteDuplicate',
  [BackendErrorCode.buttonNotFound]: 'request.backend.buttonNotFound',
  [BackendErrorCode.buttonAlreadyExists]: 'request.backend.buttonAlreadyExists',
  [BackendErrorCode.buttonCodeDuplicate]: 'request.backend.buttonCodeDuplicate',
  [BackendErrorCode.apiNotFound]: 'request.backend.apiNotFound',
  [BackendErrorCode.apiAlreadyExists]: 'request.backend.apiAlreadyExists',
  [BackendErrorCode.apiPathDuplicate]: 'request.backend.apiPathDuplicate'
};

export function getBackendErrorMessage(code: string) {
  const i18nKey = code ? backendErrorI18nKeyMap[code] : '';
  return $t(i18nKey || 'request.backend.unknown');
}
