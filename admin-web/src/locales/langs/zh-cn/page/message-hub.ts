const messageHub = {
  template: {
    title: '消息模板',
    code: '模板编码',
    name: '模板名称',
    channel: '发送通道',
    msgTitle: '消息标题',
    content: '模板内容',
    providerTplId: '第三方模板ID',
    status: '状态',
    time: '创建时间',
    test: '测试发送',
    form: {
      codePlaceholder: '请输入模板编码',
      namePlaceholder: '请输入模板名称',
      channelPlaceholder: '请选择发送通道',
      titlePlaceholder: '请输入消息标题',
      contentPlaceholder: '请输入模板内容',
      providerTplIdPlaceholder: '请输入第三方模板ID'
    }
  },
  record: {
    title: '发送记录',
    receiver: '接收人',
    channel: '通道',
    status: '状态',
    errorMsg: '失败原因',
    time: '发送时间',
    priority: '优先级',
    retryCount: '重试次数',
    detail: '详情',
    resend: '重发',
    sendSuccess: '发送成功',
    sendFailed: '发送失败',
    pending: '等待发送'
  },
  channel: {
    sms: '短信',
    email: '邮件',
    internal: '站内信',
    push: '推送'
  },
  priority: {
    high: '高',
    medium: '中',
    low: '低'
  }
};

export default messageHub;
