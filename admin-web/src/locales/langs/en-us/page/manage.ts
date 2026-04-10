const manage = {
  common: {
    status: {
      enable: 'Enable',
      disable: 'Disable'
    }
  },
  role: {
    title: 'Role List',
    roleName: 'Role Name',
    roleCode: 'Role Code',
    roleStatus: 'Role Status',
    roleDesc: 'Role Description',
    menuAuth: 'Menu Auth',
    buttonAuth: 'Button Auth',
    apiAuth: 'API Auth',
    apiMethod: 'API Method',
    apiPath: 'API Path',
    apiName: 'API Name',
    form: {
      roleName: 'Please enter role name',
      roleCode: 'Please enter role code',
      roleStatus: 'Please select role status',
      roleDesc: 'Please enter role description'
    },
    addRole: 'Add Role',
    editRole: 'Edit Role'
  },
  admin: {
    title: 'Admin List',
    userName: 'User Name',
    userGender: 'Gender',
    nickName: 'Nick Name',
    userPhone: 'Phone Number',
    userEmail: 'Email',
    userStatus: 'User Status',
    userRole: 'User Role',
    form: {
      userName: 'Please enter user name',
      userGender: 'Please select gender',
      nickName: 'Please enter nick name',
      userPhone: 'Please enter phone number',
      userEmail: 'Please enter email',
      userStatus: 'Please select user status',
      userRole: 'Please select user role'
    },
    addAdmin: 'Add Admin',
    editAdmin: 'Edit Admin',
    gender: {
      male: 'Male',
      female: 'Female'
    }
  },
  menu: {
    home: 'Home',
    title: 'Menu List',
    id: 'ID',
    parentId: 'Parent ID',
    menuType: 'Menu Type',
    menuName: 'Menu Name',
    routeName: 'Route Name',
    routePath: 'Route Path',
    pathParam: 'Path Param',
    layout: 'Layout Component',
    page: 'Page Component',
    i18nKey: 'I18n Key',
    icon: 'Icon',
    localIcon: 'Local Icon',
    iconTypeTitle: 'Icon Type',
    order: 'Order',
    constant: 'Constant',
    keepAlive: 'Keep Alive',
    href: 'Href',
    hideInMenu: 'Hide In Menu',
    activeMenu: 'Active Menu',
    multiTab: 'Multi Tab',
    fixedIndexInTab: 'Fixed Index In Tab',
    query: 'Query Params',
    button: 'Button',
    buttonCode: 'Button Code',
    buttonDesc: 'Button Desc',
    menuStatus: 'Menu Status',
    form: {
      home: 'Please select home',
      menuType: 'Please select menu type',
      menuName: 'Please enter menu name',
      routeName: 'Please enter route name',
      routePath: 'Please enter route path',
      pathParam: 'Please enter path param',
      page: 'Please select page component',
      layout: 'Please select layout component',
      i18nKey: 'Please enter i18n key',
      icon: 'Please enter iconify name',
      localIcon: 'Please enter local icon name',
      order: 'Please enter order',
      keepAlive: 'Please select whether to cache route',
      href: 'Please enter href',
      hideInMenu: 'Please select whether to hide menu',
      activeMenu: 'Please select route name of the highlighted menu',
      multiTab: 'Please select whether to support multiple tabs',
      fixedInTab: 'Please select whether to fix in the tab',
      fixedIndexInTab: 'Please enter the index fixed in the tab',
      queryKey: 'Please enter route parameter Key',
      queryValue: 'Please enter route parameter Value',
      button: 'Please select whether it is a button',
      buttonCode: 'Please enter button code',
      buttonDesc: 'Please enter button description',
      menuStatus: 'Please select menu status'
    },
    addMenu: 'Add Menu',
    editMenu: 'Edit Menu',
    addChildMenu: 'Add Child Menu',
    type: {
      directory: 'Directory',
      menu: 'Menu',
      button: 'Button'
    },
    iconType: {
      iconify: 'Iconify Icon',
      local: 'Local Icon'
    }
  },
  storage: {
    configTitle: 'Storage Config List',
    configName: 'Config Name',
    provider: 'Provider',
    endpoint: 'Endpoint',
    region: 'Region',
    bucket: 'Bucket',
    accessKey: 'AccessKey',
    secretKey: 'SecretKey',
    secretKeyPlaceholder: 'Leave empty if not changing',
    domain: 'Custom Domain',
    pathPrefix: 'Path Prefix',
    isDefault: 'Default',
    status: 'Status',
    maxFileSize: 'Max File Size',
    allowedTypes: 'Allowed Types',
    allowedTypesPlaceholder: 'e.g. jpg,png,gif,pdf separated by comma',
    stsExpireTime: 'STS Expire Time',
    remark: 'Remark',
    setDefault: 'Set Default',
    addConfig: 'Add Storage Config',
    editConfig: 'Edit Storage Config'
  },
  upload: {
    title: 'Upload Record List',
    preview: 'Preview',
    fileName: 'File Name',
    fileSize: 'File Size',
    source: 'Source',
    businessType: 'Business Type',
    storageName: 'Storage Config',
    uploaderIp: 'Uploader IP',
    uploadedAt: 'Uploaded At',
    view: 'View',
    sourceAdmin: 'Admin',
    sourceClient: 'Client',
    sourceApi: 'API',
    sourceSystem: 'System'
  },
  setting: {
    title: 'Basic Settings',
    tabs: {
      cache: 'Cache Config',
      task: 'Task Config',
      log: 'Log Maintenance'
    },
    cache: {
      title: 'Cache Degradation Switches',
      description:
        'Switches used to dynamically degrade system capabilities. Enabling cache improves performance; disabling it forces database queries. Changes trigger hot reload across Redis cluster.',
      systemGroup: 'System Core Cache',
      moduleGroup: 'Business Module Cache',
      sys_config: 'System Config/Hot reload',
      rbac_auth: 'RBAC Auth/API Permission',
      rbac_menu: 'Menu Tree/Recursive Cache',
      admin: 'Admin Profile Cache',
      dict: 'Dictionary Data Cache',
      storage: 'Object Storage Config/Hot Reload',
      err_log_cache: 'Error Log Analysis Cache',
      content_category_cache: 'Content Category Tree Cache'
    },
    log: {
      title: 'Log Retention Policies',
      description:
        'Configure the automatic cleanup cycle for system logs. Periodic cleanup is recommended to free storage space. Set to 0 for permanent retention.',
      taskLog: 'Task Scheduling Log',
      operationLog: 'Admin Operation Log',
      errorLog: 'System Error Log',
      enabled: 'Logging Enabled',
      retentionDays: 'Retention Days (0 for Permanent)',
      daysUnit: 'Days'
    }
  },
  dict: {
    typeTitle: 'Dict Types',
    dataTitle: 'Dict Data',
    typeName: 'Type Name',
    typeCode: 'Type Code',
    description: 'Description',
    dataLabel: 'Label',
    dataValue: 'Value',
    tagType: 'Tag Type',
    orderBy: 'Sort',
    remark: 'Remark',
    selectTypeFirst: 'Please select a dict type from the left first',
    addType: 'Add Dict Type',
    editType: 'Edit Dict Type',
    addData: 'Add Dict Data',
    editData: 'Edit Dict Data'
  }
};

export default manage;
