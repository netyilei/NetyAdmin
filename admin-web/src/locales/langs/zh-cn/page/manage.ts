const manage = {
  common: {
    status: {
      enable: '启用',
      disable: '禁用'
    }
  },
  role: {
    title: '角色列表',
    roleName: '角色名称',
    roleCode: '角色编码',
    roleStatus: '角色状态',
    roleDesc: '角色描述',
    menuAuth: '菜单权限',
    buttonAuth: '按钮权限',
    apiAuth: 'API权限',
    apiMethod: '请求方式',
    apiPath: '接口路径',
    apiName: '接口名称',
    form: {
      roleName: '请输入角色名称',
      roleCode: '请输入角色编码',
      roleStatus: '请选择角色状态',
      roleDesc: '请输入角色描述'
    },
    addRole: '新增角色',
    editRole: '编辑角色'
  },
  admin: {
    title: '管理员列表',
    userName: '用户名',
    userGender: '性别',
    nickName: '昵称',
    userPhone: '手机号',
    userEmail: '邮箱',
    userStatus: '用户状态',
    userRole: '用户角色',
    form: {
      userName: '请输入用户名',
      userGender: '请选择性别',
      nickName: '请输入昵称',
      userPhone: '请输入手机号',
      userEmail: '请输入邮箱',
      userStatus: '请选择用户状态',
      userRole: '请选择用户角色'
    },
    addAdmin: '新增管理员',
    editAdmin: '编辑管理员',
    gender: {
      male: '男',
      female: '女'
    }
  },
  menu: {
    home: '首页',
    title: '菜单列表',
    id: 'ID',
    parentId: '父级菜单ID',
    menuType: '菜单类型',
    menuName: '菜单名称',
    routeName: '路由名称',
    routePath: '路由路径',
    pathParam: '路径参数',
    layout: '布局',
    page: '页面组件',
    i18nKey: '国际化key',
    icon: '图标',
    localIcon: '本地图标',
    iconTypeTitle: '图标类型',
    order: '排序',
    constant: '常量路由',
    keepAlive: '缓存路由',
    href: '外链',
    hideInMenu: '隐藏菜单',
    activeMenu: '高亮的菜单',
    multiTab: '支持多页签',
    fixedIndexInTab: '固定在页签中的序号',
    query: '路由参数',
    button: '按钮',
    buttonCode: '按钮编码',
    buttonDesc: '按钮描述',
    menuStatus: '菜单状态',
    form: {
      home: '请选择首页',
      menuType: '请选择菜单类型',
      menuName: '请输入菜单名称',
      routeName: '请输入路由名称',
      routePath: '请输入路由路径',
      pathParam: '请输入路径参数',
      page: '请选择页面组件',
      layout: '请选择布局组件',
      i18nKey: '请输入国际化key',
      icon: '请输入图标',
      localIcon: '请选择本地图标',
      order: '请输入排序',
      keepAlive: '请选择是否缓存路由',
      href: '请输入外链',
      hideInMenu: '请选择是否隐藏菜单',
      activeMenu: '请选择高亮的菜单的路由名称',
      multiTab: '请选择是否支持多标签',
      fixedInTab: '请选择是否固定在页签中',
      fixedIndexInTab: '请输入固定在页签中的序号',
      queryKey: '请输入路由参数Key',
      queryValue: '请输入路由参数Value',
      button: '请选择是否按钮',
      buttonCode: '请输入按钮编码',
      buttonDesc: '请输入按钮描述',
      menuStatus: '请选择菜单状态'
    },
    addMenu: '新增菜单',
    editMenu: '编辑菜单',
    addChildMenu: '新增子菜单',
    type: {
      dir: '目录',
      menu: '菜单',
      button: '按钮'
    },
    iconType: {
      iconify: 'iconify图标',
      local: '本地图标'
    }
  },
  storage: {
    configTitle: '存储配置列表',
    configName: '配置名称',
    provider: '存储提供商',
    endpoint: '服务端点',
    region: '区域',
    bucket: '存储桶',
    accessKey: 'AccessKey',
    secretKey: 'SecretKey',
    secretKeyPlaceholder: '不修改请留空',
    domain: '自定义域名',
    pathPrefix: '路径前缀',
    isDefault: '默认配置',
    status: '状态',
    maxFileSize: '最大文件大小',
    allowedTypes: '允许的文件类型',
    allowedTypesPlaceholder: '如: jpg,png,gif,pdf 多个用逗号分隔',
    stsExpireTime: 'STS过期时间',
    remark: '备注',
    setDefault: '设为默认',
    addConfig: '新增存储配置',
    editConfig: '编辑存储配置'
  },
  upload: {
    title: '上传记录列表',
    preview: '预览',
    fileName: '文件名',
    fileSize: '文件大小',
    source: '上传来源',
    businessType: '业务类型',
    storageName: '存储配置',
    uploaderIp: '上传IP',
    uploadedAt: '上传时间',
    view: '查看',
    sourceAdmin: '管理后台',
    sourceClient: '客户端',
    sourceApi: 'API',
    sourceSystem: '系统'
  },
  setting: {
    title: '基础设置',
    tabs: {
      cache: '缓存配置',
      task: '任务配置',
      log: '日志管理'
    },
    cache: {
      title: '缓存降级开关',
      description:
        '这里的开关用于动态降级系统能力。开启缓存后将极大提升系统性能；关闭缓存则强制直接查询数据库（用于紧急排错）。修改后将自动通过 Redis 集群广播热重载。',
      systemGroup: '系统核心层缓存',
      moduleGroup: '业务模块级缓存',
      sys_config: '系统基础配置/热更中心',
      rbac_auth: 'RBAC 权限/API 鉴权',
      rbac_menu: '菜单树格式化/递归缓存',
      admin: '管理员个人资料缓存',
      dict: '字典数据缓存',
      storage: '对象存储配置/驱动热更',
      err_log_cache: '错误日志聚合分析缓存',
      content_category_cache: '内容分类(无限级树)缓存'
    },
    log: {
      title: '日志保留策略',
      description: '配置系统日志的自动清理周期。建议定期清理以释放存储空间。设置为 0 则表示永久保留。',
      taskLog: '任务调度日志',
      operationLog: '管理员操作日志',
      errorLog: '系统错误日志',
      enabled: '记录开关',
      retentionDays: '保留天数(0为永久)',
      daysUnit: '天'
    }
  },
  dict: {
    typeTitle: '字典类型',
    dataTitle: '字典数据',
    typeName: '字典名称',
    typeCode: '字典编码',
    description: '备注',
    dataLabel: '字典标签',
    dataValue: '字典键值',
    tagType: '标签类型',
    orderBy: '排序',
    remark: '备注',
    selectTypeFirst: '请先从左侧选择一个字典类型',
    addType: '新增字典类型',
    editType: '编辑字典类型',
    addData: '新增字典数据',
    editData: '编辑字典数据'
  }
};

export default manage;
