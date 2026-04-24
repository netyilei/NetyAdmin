export const ENABLE_STATUS = {
  ENABLED: '1',
  DISABLED: '0'
} as const;

export const GENDER = {
  MALE: '1',
  FEMALE: '2'
} as const;

export const MENU_TYPE = {
  DIRECTORY: '1',
  MENU: '2',
  BUTTON: '3'
} as const;

export const ICON_TYPE = {
  ICONIFY: '1',
  LOCAL: '2'
} as const;

export const DICT_BOOLEAN = {
  TRUE: '1',
  FALSE: '0'
} as const;

export function isEnabledStatus(status: string | null | undefined): boolean {
  return status === ENABLE_STATUS.ENABLED;
}

export function isDisabledStatus(status: string | null | undefined): boolean {
  return status === ENABLE_STATUS.DISABLED;
}

export function boolToDictValue(val: boolean | null | undefined): string {
  if (val === null || val === undefined) return DICT_BOOLEAN.FALSE;
  return val ? DICT_BOOLEAN.TRUE : DICT_BOOLEAN.FALSE;
}

export function dictValueToBool(val: string | number | null | undefined): boolean {
  return String(val) === DICT_BOOLEAN.TRUE;
}

export function isTruthyConfigValue(val: string | null | undefined): boolean {
  if (val === null || val === undefined) return false;
  return val === 'true' || val === ENABLE_STATUS.ENABLED;
}
