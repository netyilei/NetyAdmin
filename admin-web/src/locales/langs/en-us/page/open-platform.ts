const openPlatform = {
  app: {
    title: 'App Management',
    name: 'App Name',
    appKey: 'AppKey',
    appSecret: 'AppSecret',
    status: 'Status',
    ipFilterEnabled: 'IP Filter',
    rateLimitEnabled: 'Rate Limit',
    ipRules: 'IP Rules',
    remark: 'Remark',
    time: 'Created At',
    scopes: 'Scopes',
    storageId: 'Storage Config',
    storageBound: 'Bound',
    storageDefault: 'Default',
    quotaConfig: 'Rate Limit',
    quotaRate: 'Requests/sec',
    quotaCapacity: 'Burst Limit',
    quotaDefault: 'Default',
    cacheTTL: 'Cache TTL',
    resetSecret: 'Reset Secret',
    confirmResetSecret: 'Are you sure to reset the AppSecret? The old key will be invalidated immediately!',
    resetSecretSuccess: 'Secret reset successful. Please keep the new secret safe: ',
    resetSecretWarning:
      'Please keep this secret safe! You will not be able to view it again after closing this window.',
    form: {
      namePlaceholder: 'Please enter app name',
      appKeyPlaceholder: 'Please enter AppKey',
      statusPlaceholder: 'Please select status',
      ipRulesPlaceholder: 'Please select IP rules',
      remarkPlaceholder: 'Please enter remark',
      scopesPlaceholder: 'Please select scopes',
      storageIdPlaceholder: 'Please select storage config',
      storageIdDefault: 'Use Default',
      quotaRatePlaceholder: 'Leave empty for system default',
      quotaRateSuffix: 'req/sec',
      quotaRateTip: 'Maximum requests per second. Requests exceeding this rate will be throttled',
      quotaCapacityPlaceholder: 'Leave empty for system default',
      quotaCapacitySuffix: 'req',
      quotaCapacityTip: 'Maximum concurrent requests allowed in a short burst to handle traffic spikes',
      cacheTTLPlaceholder: '0 for system default',
      cacheTTLSuffix: 'sec',
      cacheTTLTip: 'Cache expiration time for this app. Set to 0 to use system default (local_ttl_min)'
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
  },
  ipac: {
    title: 'IP Access Control',
    id: 'ID',
    appId: 'App',
    global: 'Global',
    ipAddr: 'IP/CIDR',
    type: 'Action',
    typeAllow: 'Allow',
    typeDeny: 'Deny',
    reason: 'Reason',
    expiredAt: 'Expires At',
    status: 'Status',
    permanent: 'Permanent',
    time: 'Created At',
    operator: 'Operator',
    form: {
      ipAddrPlaceholder: 'Enter single IP or CIDR range',
      reasonPlaceholder: 'Enter reason',
      expiredAtPlaceholder: 'Select expiry time (empty for permanent)',
      typePlaceholder: 'Select action type',
      statusPlaceholder: 'Select status'
    }
  }
};

export default openPlatform;
