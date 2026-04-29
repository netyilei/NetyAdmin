const openPlatform = {
  app: {
    title: '应用管理',
    name: '应用名称',
    appKey: 'AppKey',
    appSecret: 'AppSecret',
    status: '状态',
    ipFilterEnabled: 'IP 过滤',
    rateLimitEnabled: '限流开关',
    ipRules: 'IP 规则',
    remark: '备注',
    time: '创建时间',
    scopes: '权限范围',
    storageId: '存储配置',
    storageBound: '已绑定',
    storageDefault: '默认',
    quotaConfig: '限流配置',
    quotaRate: '每秒请求数',
    quotaCapacity: '突发上限',
    quotaDefault: '默认',
    cacheTTL: '缓存有效期',
    resetSecret: '重置密钥',
    confirmResetSecret: '确定重置该应用的 AppSecret 吗？重置后旧密钥将立即失效！',
    resetSecretSuccess: '密钥重置成功，请妥善保管新密钥：',
    resetSecretWarning: '请务必妥善保管，关闭此窗口后将无法再次查看！',
    form: {
      namePlaceholder: '请输入应用名称',
      appKeyPlaceholder: '请输入 AppKey',
      statusPlaceholder: '请选择状态',
      ipRulesPlaceholder: '请选择 IP 规则',
      remarkPlaceholder: '请输入备注',
      scopesPlaceholder: '请选择权限范围',
      storageIdPlaceholder: '请选择存储配置',
      storageIdDefault: '使用默认',
      quotaRatePlaceholder: '留空使用系统默认',
      quotaRateSuffix: '次/秒',
      quotaRateTip: '该应用每秒允许处理的最大请求数，超过此速率的请求将被限流',
      quotaCapacityPlaceholder: '留空使用系统默认',
      quotaCapacitySuffix: '次',
      quotaCapacityTip: '允许短时间内的最大并发请求数，用于应对瞬时流量高峰',
      cacheTTLPlaceholder: '0 为永久缓存',
      cacheTTLSuffix: '秒',
      cacheTTLTip: '该应用配置的缓存过期时间，设为 0 则永久缓存，仅通过更新应用时自动失效'
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
  },
  ipac: {
    title: 'IP 访问控制',
    id: 'ID',
    appId: '所属应用',
    global: '全局规则',
    ipAddr: 'IP 地址/网段',
    type: '动作类型',
    typeAllow: '放行',
    typeDeny: '封禁',
    reason: '原因',
    expiredAt: '过期时间',
    status: '状态',
    permanent: '永久有效',
    time: '创建时间',
    operator: '操作人',
    form: {
      ipAddrPlaceholder: '请输入单个 IP 或 CIDR 网段',
      reasonPlaceholder: '请输入操作原因',
      expiredAtPlaceholder: '请选择过期时间（留空为永久）',
      typePlaceholder: '请选择动作类型',
      statusPlaceholder: '请选择状态'
    }
  }
};

export default openPlatform;
