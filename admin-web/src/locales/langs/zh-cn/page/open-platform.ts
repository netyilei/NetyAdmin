const openPlatform = {
  app: {
    title: '应用管理',
    name: '应用名称',
    appKey: 'AppKey',
    appSecret: 'AppSecret',
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
      statusPlaceholder: '请选择状态',
      ipStrategyPlaceholder: '请选择 IP 策略',
      remarkPlaceholder: '请输入备注',
      scopesPlaceholder: '请选择权限范围'
    }
  },
  scope: {
    title: '接口权限',
    name: '权限标识',
    displayName: '显示名称',
    description: '权限说明',
    bindApis: '关联API',
    form: {
      displayNamePlaceholder: '请输入权限显示名称'
    }
  },
  api: {
    title: 'API管理',
    method: '请求方法',
    path: '请求路径',
    name: 'API名称',
    group: '分组',
    status: '状态',
    description: '描述',
    time: '创建时间',
    form: {
      pathPlaceholder: '例如: /api/v1/users',
      namePlaceholder: '例如: 获取用户列表',
      groupPlaceholder: '例如: user，默认: default'
    }
  }
};

export default openPlatform;
