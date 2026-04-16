const openPlatform = {
  app: {
    title: '应用管理',
    name: '应用名称',
    appKey: 'AppKey',
    appSecret: 'AppSecret',
    type: '应用类型',
    typeInternal: '官方内部',
    typeExternal: '外部合作伙伴',
    status: '状态',
    ipStrategy: 'IP 策略',
    ipStrategyBlacklist: '黑名单模式',
    ipStrategyWhitelist: '白名单模式',
    remark: '备注',
    time: '创建时间',
    scopes: '权限范围',
    resetSecret: '重置密钥',
    confirmResetSecret: '确定重置该应用的 AppSecret 吗？重置后旧密钥将立即失效！',
    resetSecretSuccess: '密钥重置成功，请妥善保管新密钥：',
    form: {
      namePlaceholder: '请输入应用名称',
      appKeyPlaceholder: '请输入 AppKey',
      typePlaceholder: '请选择应用类型',
      statusPlaceholder: '请选择状态',
      ipStrategyPlaceholder: '请选择 IP 策略',
      remarkPlaceholder: '请输入备注',
      scopesPlaceholder: '请选择权限范围'
    }
  },
  scope: {
    title: '接口权限',
    name: '权限标识',
    description: '权限说明',
    userBase: '用户基础 (注册/登录)',
    userProfile: '用户资料 (修改/注销)',
    msgSend: '消息发送 (SMS/Email)',
    contentView: '内容查看'
  }
};

export default openPlatform;
