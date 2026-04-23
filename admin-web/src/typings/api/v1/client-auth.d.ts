export namespace ClientAuth {
  /** captcha result */
  interface CaptchaResult {
    captchaId: string;
    img: string;
  }

  /** send code request */
  interface SendCodeReq {
    /** register, reset_password, login */
    scene: string;
    /** phone or email */
    target: string;
    captchaKey: string;
    captchaCode: string;
  }

  /** scene config (merged captcha-status + verify-config) */
  interface SceneConfig {
    scene: string;
    captchaEnabled: boolean;
    verifyEnabled: boolean;
    verifyType: 'email' | 'sms' | '';
  }
}
