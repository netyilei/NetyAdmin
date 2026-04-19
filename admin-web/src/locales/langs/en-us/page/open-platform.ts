const openPlatform = {
  app: {
    title: 'App Management',
    name: 'App Name',
    appKey: 'AppKey',
    appSecret: 'AppSecret',
    status: 'Status',
    ipFilterEnabled: 'IP Filter',
    ipRules: 'IP Rules',
    remark: 'Remark',
    time: 'Created At',
    scopes: 'Scopes',
    resetSecret: 'Reset Secret',
    confirmResetSecret: 'Are you sure to reset the AppSecret? The old key will be invalidated immediately!',
    resetSecretSuccess: 'Secret reset successful. Please keep the new secret safe: ',
    form: {
      namePlaceholder: 'Please enter app name',
      appKeyPlaceholder: 'Please enter AppKey',
      statusPlaceholder: 'Please select status',
      ipRulesPlaceholder: 'Please select IP rules',
      remarkPlaceholder: 'Please enter remark',
      scopesPlaceholder: 'Please select scopes'
    }
  },
  scope: {
    title: 'API Scopes',
    name: 'Scope Name',
    displayName: 'Display Name',
    description: 'Description',
    bindApis: 'Bind APIs',
    form: {
      displayNamePlaceholder: 'Please enter scope display name'
    }
  },
  api: {
    title: 'API Management',
    method: 'Method',
    path: 'Path',
    name: 'API Name',
    group: 'Group',
    status: 'Status',
    description: 'Description',
    time: 'Created At',
    form: {
      pathPlaceholder: 'e.g. /api/v1/users',
      namePlaceholder: 'e.g. Get User List',
      groupPlaceholder: 'e.g. user, default: default'
    }
  }
};

export default openPlatform;
