export namespace ClientAuth {
  /** captcha result */
  interface CaptchaResult {
    captchaId: string;
    img: string;
  }

  /** send code request */
  interface SendCodeReq {
    /** register, reset_password */
    scene: string;
    /** phone or email */
    target: string;
    captchaId: string;
    captcha: string;
  }

  /** verify config */
  interface VerifyConfig {
    enabled: boolean;
    verifyType: 'email' | 'sms';
    scene: string;
  }
}
